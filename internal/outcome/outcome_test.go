// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package outcome

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const tasksRoot = "../../benchmark/outcomes/testdata"

func TestLoad_TaskSetShape(t *testing.T) {
	tasks, err := Load(tasksRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) < 40 {
		t.Fatalf("want at least 40 outcome tasks, got %d", len(tasks))
	}
	perLabel := map[string]int{}
	for _, task := range tasks {
		perLabel[task.Label]++
		for _, f := range []string{"stub.go", "reference.go", "check_test.go"} {
			if _, err := os.Stat(filepath.Join(task.Dir, f)); err != nil {
				t.Errorf("%s: %v", task.ID, err)
			}
		}
	}
	for _, l := range []string{"TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"} {
		if perLabel[l] < 8 {
			t.Errorf("label %s has %d tasks, want >= 8", l, perLabel[l])
		}
	}
}

func TestWilson(t *testing.T) {
	cases := []struct {
		k, n   int
		lo, hi float64
	}{{5, 10, 0.2366, 0.7634}, {0, 10, 0, 0.2775}, {10, 10, 0.7225, 1}, {36, 40, 0.7695, 0.9604}}
	for _, c := range cases {
		lo, hi := Wilson(c.k, c.n)
		if math.Abs(lo-c.lo) > 1e-3 || math.Abs(hi-c.hi) > 1e-3 {
			t.Errorf("Wilson(%d,%d)=[%.4f,%.4f] want [%.4f,%.4f]", c.k, c.n, lo, hi, c.lo, c.hi)
		}
	}
}

func TestExtractGo(t *testing.T) {
	if got := ExtractGo("Sure!\n```go\npackage task\n```\nbye"); got != "package task\n" {
		t.Fatalf("got %q", got)
	}
	if got := ExtractGo("package task\n"); got != "package task\n" {
		t.Fatalf("unfenced answer must pass through, got %q", got)
	}
}

func TestSplice(t *testing.T) {
	doc := "a\n" + BeginMarker + "\nold\n" + EndMarker + "\nz\n"
	got, err := Splice(doc, BeginMarker+"\nnew\n"+EndMarker+"\n")
	if err != nil || got != "a\n"+BeginMarker+"\nnew\n"+EndMarker+"\nz\n" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := Splice("no markers", "x"); err == nil {
		t.Fatal("missing markers must be an error")
	}
}

func TestReport_NoRunsSaysNotMeasured(t *testing.T) {
	tasks, _ := Load(tasksRoot)
	out, err := Report(nil, tasks, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "**not measured**") || strings.Contains(out, "| Run tier") {
		t.Fatalf("without recorded runs the report must not show pass rates:\n%s", out)
	}
}

func TestReport_WithRuns(t *testing.T) {
	tasks := []Task{
		{ID: "a", Label: "TRIVIAL", Prompt: "fix typo in README"},
		{ID: "b", Label: "SIMPLE", Prompt: "rename variable foo to bar"},
		{ID: "c", Label: "COMPLEX", Prompt: "design a distributed consensus protocol for multi-region replication"},
	}
	runs := []Run{
		{Tier: "frontier", Model: "big", Date: "2026-10-03", Results: map[string]bool{"a": true, "b": true, "c": true}},
		{Tier: "small", Model: "tiny", Date: "2026-10-03", Results: map[string]bool{"a": true, "b": false, "c": false}},
	}
	out, err := Report(nil, tasks, runs)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"| small | `tiny` | 2026-10-03 | 1/3 = 33.3%", "| frontier | `big` |", "TRIVIAL+SIMPLE labels: -50.0pp"} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
}

// writeRun records a fake run whose solutions are copied from the task files.
func writeRun(t *testing.T, runs string, task Task, src string, claimed bool) {
	t.Helper()
	dir := filepath.Join(runs, "small")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(task.Dir, src))
	_ = os.WriteFile(filepath.Join(dir, SolutionFile(task.ID)), b, 0o644)
	run, _ := json.Marshal(Run{Tier: "small", Model: "fake", Results: map[string]bool{task.ID: claimed}})
	_ = os.WriteFile(filepath.Join(dir, "run.json"), run, 0o644)
}

func TestVerify_ChecksDiscriminateAndRecordsReproduce(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in throwaway modules")
	}
	all, _ := Load(tasksRoot)
	tasks := all[:2]
	runs := t.TempDir()
	if problems, err := Verify(context.Background(), tasks, runs); err != nil || len(problems) > 0 {
		t.Fatalf("clean tasks: %v %v", problems, err)
	}
	writeRun(t, runs, tasks[0], "stub.go", true) // a run claiming the stub passed
	problems, err := Verify(context.Background(), tasks, runs)
	if err != nil || len(problems) != 1 || !strings.Contains(problems[0], "recorded small pass=false, want true") {
		t.Fatalf("a fabricated recorded result must be caught, got %v %v", problems, err)
	}
}

func TestRecord_FakeSolver(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go test in throwaway modules")
	}
	all, _ := Load(tasksRoot)
	task := all[0]
	ref, _ := filepath.Abs(filepath.Join(task.Dir, "reference.go"))
	runs := t.TempDir()
	solver := "cat >/dev/null; echo 'here you go'; echo '```go'; cat " + ref + "; echo '```'"
	run, err := Record(context.Background(), []Task{task}, "small", "fake", solver, runs, func(string, ...any) {})
	if err != nil || !run.Results[task.ID] {
		t.Fatalf("record: %+v %v", run, err)
	}
	loaded, err := LoadRuns(runs)
	if err != nil || len(loaded) != 1 || loaded[0].Model != "fake" || !loaded[0].Results[task.ID] {
		t.Fatalf("LoadRuns: %+v %v", loaded, err)
	}
	// Recorded solutions must not be .go files, or the repo's own go build
	// would compile model output committed under benchmark/outcomes/runs.
	if _, err := os.Stat(filepath.Join(runs, "small", task.ID+".go")); err == nil {
		t.Fatalf("Record wrote %s.go; recorded solutions must use %s", task.ID, SolutionFile(task.ID))
	}
	if _, err := os.Stat(filepath.Join(runs, "small", SolutionFile(task.ID))); err != nil {
		t.Fatalf("recorded solution missing: %v", err)
	}
}
