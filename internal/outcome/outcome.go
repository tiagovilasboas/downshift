// Package outcome is the outcome eval: coding tasks with an executable check,
// solved by a model on a given tier and scored by running the check. It is
// the quality half of the router's cost/quality trade-off; the classifier
// benchmark only measures agreement with human complexity labels.
//
// Model calls happen only in Record, through a user-supplied solver command.
// Everything else (Verify, LoadRuns, Report) is offline and runs in CI.
package outcome

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// Task is one directory under the tasks root: task.json, stub.go (starting
// point shown to the model), reference.go (known-good solution) and
// check_test.go (hidden executable check).
type Task struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Prompt string `json:"prompt"`
	Dir    string `json:"-"`
}

// Run is one recorded pass over the tasks with one model.
type Run struct {
	Tier    string          `json:"tier"`
	Model   string          `json:"model"`
	Date    string          `json:"date"`
	Results map[string]bool `json:"results"`
}

// Load reads every task under root, sorted by ID.
func Load(root string) ([]Task, error) {
	metas, err := filepath.Glob(filepath.Join(root, "*", "task.json"))
	if err != nil {
		return nil, err
	}
	var tasks []Task
	for _, m := range metas {
		b, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		var t Task
		if err := json.Unmarshal(b, &t); err != nil {
			return nil, fmt.Errorf("%s: %w", m, err)
		}
		t.Dir = filepath.Dir(m)
		if t.ID != filepath.Base(t.Dir) || t.Prompt == "" {
			return nil, fmt.Errorf("%s: id must match directory and prompt must be set", m)
		}
		tasks = append(tasks, t)
	}
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
	return tasks, nil
}

// Check runs the task's hidden check against solution in a throwaway module.
func Check(ctx context.Context, t Task, solution []byte) (bool, error) {
	dir, err := os.MkdirTemp("", "outcome-"+t.ID+"-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(dir)
	check, err := os.ReadFile(filepath.Join(t.Dir, "check_test.go"))
	if err != nil {
		return false, err
	}
	files := map[string][]byte{"go.mod": []byte("module task\n\ngo 1.22\n"), "solution.go": solution, "check_test.go": check}
	for name, b := range files {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return false, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	err = cmd.Run()
	if _, isExit := err.(*exec.ExitError); err != nil && !isExit {
		return false, err
	}
	return err == nil, nil
}

// SolverPrompt is what the solver command receives on stdin.
func SolverPrompt(t Task) (string, error) {
	stub, err := os.ReadFile(filepath.Join(t.Dir, "stub.go"))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("You are solving a Go coding task. Use only the standard library.\n"+
		"Reply with the complete contents of solution.go (package task) in one ```go fenced block.\n\n"+
		"Task: %s\n\nStarting point (stub.go):\n```go\n%s```\n", t.Prompt, stub), nil
}

var fence = regexp.MustCompile("(?s)```(?:go|golang)?\\s*\\n(.*?)```")

// ExtractGo returns the first fenced code block of out, or out itself.
func ExtractGo(out string) string {
	if m := fence.FindStringSubmatch(out); m != nil {
		return m[1]
	}
	return out
}

// Record solves every task with solver (run through `sh -c`, prompt on stdin,
// answer on stdout), stores each solution as <runsDir>/<tier>/<id>.go, checks
// it and writes run.json next to the solutions.
func Record(ctx context.Context, tasks []Task, tier, model, solver, runsDir string, log func(string, ...any)) (Run, error) {
	dir := filepath.Join(runsDir, tier)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Run{}, err
	}
	run := Run{Tier: tier, Model: model, Date: time.Now().UTC().Format("2006-01-02"), Results: map[string]bool{}}
	for _, t := range tasks {
		prompt, err := SolverPrompt(t)
		if err != nil {
			return run, err
		}
		cmd := exec.CommandContext(ctx, "sh", "-c", solver)
		cmd.Stdin = strings.NewReader(prompt)
		var stdout bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return run, fmt.Errorf("solver failed on %s: %w", t.ID, err)
		}
		sol := []byte(ExtractGo(stdout.String()))
		if err := os.WriteFile(filepath.Join(dir, t.ID+".go"), sol, 0o644); err != nil {
			return run, err
		}
		pass, err := Check(ctx, t, sol)
		if err != nil {
			return run, err
		}
		run.Results[t.ID] = pass
		log("%s %s pass=%v\n", tier, t.ID, pass)
	}
	b, _ := json.MarshalIndent(run, "", "  ")
	return run, os.WriteFile(filepath.Join(dir, "run.json"), append(b, '\n'), 0o644)
}

// LoadRuns reads every <runsDir>/<tier>/run.json. A missing runsDir is not an error.
func LoadRuns(runsDir string) ([]Run, error) {
	metas, _ := filepath.Glob(filepath.Join(runsDir, "*", "run.json"))
	var runs []Run
	for _, m := range metas {
		b, err := os.ReadFile(m)
		if err != nil {
			return nil, err
		}
		var r Run
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, fmt.Errorf("%s: %w", m, err)
		}
		if r.Tier != filepath.Base(filepath.Dir(m)) {
			return nil, fmt.Errorf("%s: tier %q does not match its directory", m, r.Tier)
		}
		runs = append(runs, r)
	}
	return runs, nil
}

// Verify proves every check is executable and discriminating (the stub fails,
// the reference passes) and that every recorded result is reproducible from
// its recorded solution. It returns one line per problem.
func Verify(ctx context.Context, tasks []Task, runsDir string) ([]string, error) {
	runs, err := LoadRuns(runsDir)
	if err != nil {
		return nil, err
	}
	type job struct {
		t        Task
		file     string
		want     bool
		describe string
	}
	var jobs []job
	for _, t := range tasks {
		jobs = append(jobs, job{t, filepath.Join(t.Dir, "stub.go"), false, "stub"},
			job{t, filepath.Join(t.Dir, "reference.go"), true, "reference"})
		for _, r := range runs {
			if want, ok := r.Results[t.ID]; ok {
				jobs = append(jobs, job{t, filepath.Join(runsDir, r.Tier, t.ID+".go"), want, "recorded " + r.Tier})
			}
		}
	}
	var mu sync.Mutex
	var problems []string
	var firstErr error
	sem := make(chan struct{}, runtime.NumCPU())
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer func() { <-sem; wg.Done() }()
			src, err := os.ReadFile(j.file)
			var pass bool
			if err == nil {
				pass, err = Check(ctx, j.t, src)
			}
			mu.Lock()
			defer mu.Unlock()
			if err != nil && firstErr == nil {
				firstErr = err
			} else if err == nil && pass != j.want {
				problems = append(problems, fmt.Sprintf("%s: %s pass=%v, want %v", j.t.ID, j.describe, pass, j.want))
			}
		}(j)
	}
	wg.Wait()
	sort.Strings(problems)
	return problems, firstErr
}
