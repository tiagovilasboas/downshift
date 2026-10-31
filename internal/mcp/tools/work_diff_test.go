// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

// finishedProc is a worker that has already exited, so a diff test never
// touches a real CLI.
type finishedProc struct{}

func (finishedProc) Wait() error             { return nil }
func (finishedProc) Kill() error             { return nil }
func (finishedProc) Report() string          { return "done" }
func (finishedProc) Stderr() string          { return "" }
func (finishedProc) Tokens() *workers.Tokens { return nil }

// editingInner simulates a write worker: when launched inside its worktree, it
// changes a tracked file and adds a new one, then finishes. The WorktreeLauncher
// around it records the worktree so diff can read it.
type editingInner struct{}

func (editingInner) Launch(s workers.Spec) (workers.Process, error) {
	_ = os.WriteFile(filepath.Join(s.Dir, "kept.txt"), []byte("one\nedited\n"), 0o644)
	_ = os.WriteFile(filepath.Join(s.Dir, "fresh.txt"), []byte("a new line\n"), 0o644)
	return finishedProc{}, nil
}

// gitInit makes a repo with one committed file, the base the worktree forks.
func gitInit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Strip the caller's GIT_* (a pre-commit hook sets GIT_DIR) so git acts
		// on this temp repo, then set a deterministic identity for the commit.
		cmd.Env = append(workers.GitEnv(os.Environ()),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("one\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "kept.txt")
	run("commit", "-q", "-m", "base")
	return dir
}

// diffDeps wires a queue whose write route uses the WorktreeLauncher over the
// editing inner, so a delegated write job ends with a real worktree to diff.
// root is the project directory delegate defaults a missing dir to.
func diffDeps(t *testing.T, root string) Deps {
	t.Helper()
	write := workers.WorktreeLauncher(editingInner{})
	q := workers.New(workers.Routes(readOnlyInner{}, write), 1, time.Now)
	t.Cleanup(q.Close)
	return Deps{
		Strain:       fakeStrain{},
		Work:         q,
		WriteEnabled: true,
		Root:         root,
	}
}

// runDiffJob delegates one write job, waits for it to finish, and returns the id.
func runDiffJob(t *testing.T, d Deps) string {
	t.Helper()
	res, err := d.delegate(context.Background(), json.RawMessage(`{"tasks":["do it"],"agent":"claude","mode":"write"}`))
	if err != nil {
		t.Fatalf("delegate: %v", err)
	}
	var env struct {
		Data struct {
			Jobs []struct {
				ID string `json:"id"`
			} `json:"jobs"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(res.Text), &env); err != nil {
		t.Fatalf("delegate result: %v\n%s", err, res.Text)
	}
	if len(env.Data.Jobs) != 1 {
		t.Fatalf("expected one job, got %s", res.Text)
	}
	id := env.Data.Jobs[0].ID
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c := d.Work.Collect(context.Background(), 0, false, time.Millisecond)
		if c.Active == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	return id
}

// Happy path: diff returns the worker's change as a unified diff, inside the
// one MCP envelope, with the worker state and no truncation.
func TestDiffHandlerReturnsTheWorktreeDiff(t *testing.T) {
	// Run the whole write path in a real repo so a worktree exists.
	repo := gitInit(t)
	d := diffDeps(t, repo)
	id := runDiffJob(t, d)

	res, err := d.diff(context.Background(), json.RawMessage(`{"job_id":"`+id+`"}`))
	if err != nil {
		t.Fatalf("diff: %v", err)
	}
	var env struct {
		Data struct {
			Job       string `json:"job"`
			State     string `json:"state"`
			Diff      string `json:"diff"`
			Truncated bool   `json:"truncated"`
		} `json:"data"`
		Strain []any `json:"strain"`
	}
	if err := json.Unmarshal([]byte(res.Text), &env); err != nil {
		t.Fatalf("diff result is not the envelope: %v\n%s", err, res.Text)
	}
	if env.Data.Job != id || env.Data.State != "done" {
		t.Fatalf("job/state = %q/%q, want %q/done", env.Data.Job, env.Data.State, id)
	}
	if !strings.Contains(env.Data.Diff, "kept.txt") || !strings.Contains(env.Data.Diff, "edited") {
		t.Errorf("diff must show the modified file:\n%s", env.Data.Diff)
	}
	if !strings.Contains(env.Data.Diff, "fresh.txt") || !strings.Contains(env.Data.Diff, "a new line") {
		t.Errorf("diff must show the new file:\n%s", env.Data.Diff)
	}
	if env.Data.Truncated {
		t.Error("a small diff must not be truncated")
	}
	t.Logf("example diff tool response:\n%s", res.Text)
}

// Unhappy 1: no queue wired, the tool says workers are unavailable.
func TestDiffHandlerWithoutWorkers(t *testing.T) {
	var d Deps
	if _, err := d.diff(context.Background(), json.RawMessage(`{"job_id":"w001"}`)); err == nil {
		t.Fatal("diff must fail when workers are not available")
	}
}

// Unhappy 2: an empty job_id is refused before anything is read.
func TestDiffHandlerEmptyID(t *testing.T) {
	d := Deps{Work: workers.New(workers.Routes(nil, nil), 1, time.Now)}
	if _, err := d.diff(context.Background(), json.RawMessage(`{"job_id":"  "}`)); err == nil {
		t.Fatal("an empty job_id must be refused")
	}
	// An unknown field is refused too, so a typo cannot change the call.
	if _, err := d.diff(context.Background(), json.RawMessage(`{"job":"w001"}`)); err == nil {
		t.Fatal("an unknown field must be refused")
	}
}

// Unhappy 3: an unknown id and a read-only worker both error, no panic.
func TestDiffHandlerUnknownAndReadOnly(t *testing.T) {
	d := Deps{Strain: fakeStrain{}, Work: workers.New(workers.Routes(readOnlyInner{}, nil), 1, time.Now), Root: t.TempDir()}
	t.Cleanup(d.Work.Close)
	if _, err := d.diff(context.Background(), json.RawMessage(`{"job_id":"w404"}`)); err == nil {
		t.Fatal("an unknown id must error")
	}
	// A read-only worker has no worktree.
	res, err := d.delegate(context.Background(), json.RawMessage(`{"tasks":["read something"],"agent":"claude","mode":"read"}`))
	if err != nil {
		t.Fatalf("delegate read: %v", err)
	}
	var env struct {
		Data struct {
			Jobs []struct {
				ID string `json:"id"`
			} `json:"jobs"`
		} `json:"data"`
	}
	_ = json.Unmarshal([]byte(res.Text), &env)
	id := env.Data.Jobs[0].ID
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if c := d.Work.Collect(context.Background(), 0, false, time.Millisecond); c.Active == 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if _, err := d.diff(context.Background(), json.RawMessage(`{"job_id":"`+id+`"}`)); err == nil {
		t.Fatal("a read-only worker must have no diff")
	}
}

// readOnlyInner is a finished read-only worker: no worktree.
type readOnlyInner struct{}

func (readOnlyInner) Launch(workers.Spec) (workers.Process, error) { return finishedProc{}, nil }
