// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/workers"
)

// viewProc is a worker a test ends by hand, with a report and tokens it chooses.
type viewProc struct {
	report string
	tokens *workers.Tokens
	done   chan error
	killed chan struct{}
}

func newViewProc() *viewProc {
	return &viewProc{done: make(chan error, 1), killed: make(chan struct{})}
}

func (p *viewProc) Wait() error {
	select {
	case err := <-p.done:
		return err
	case <-p.killed:
		return context.Canceled
	}
}
func (p *viewProc) Kill() error             { close(p.killed); return nil }
func (p *viewProc) Report() string          { return p.report }
func (p *viewProc) Stderr() string          { return "" }
func (p *viewProc) Tokens() *workers.Tokens { return p.tokens }
func (p *viewProc) finish(report string, err error) {
	p.report = report
	p.done <- err
}

// viewLauncher hands out viewProcs keyed by task.
type viewLauncher struct{ procs map[string]*viewProc }

func newViewLauncher() *viewLauncher { return &viewLauncher{procs: map[string]*viewProc{}} }

func (l *viewLauncher) Launch(s workers.Spec) (workers.Process, error) {
	p := newViewProc()
	l.procs[s.Task] = p
	return p, nil
}

// statusWorkers runs the status tool and returns its decoded workers block.
func statusWorkers(t *testing.T, d Deps) *workersBlock {
	t.Helper()
	env, err := call(t, d, "status", `{}`)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var s statusData
	if err := json.Unmarshal(env["data"], &s); err != nil {
		t.Fatalf("status data is not JSON: %v", err)
	}
	return s.Workers
}

// waitWorkers polls the status tool until cond holds on its workers block.
func waitWorkers(t *testing.T, d Deps, what string, cond func(*workersBlock) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond(statusWorkers(t, d)) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Happy path: status lists the workers of this session with their id, state,
// mode, elapsed, tokens and a short summary, and names the harness only from
// the agent (no model cravado, none is recorded).
func TestStatusListsRunningAndFinishedWorkers(t *testing.T) {
	l := newViewLauncher()
	d := fullDeps()
	d.Work = workers.New(l, 4, time.Now)
	d.WriteEnabled = true
	d.Root = "/proj"

	if _, err := d.delegate(context.Background(), json.RawMessage(
		`{"tasks":["read task"],"agent":"codex","mode":"read"}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := d.delegate(context.Background(), json.RawMessage(
		`{"tasks":["write task"],"agent":"claude","mode":"write","dir":"/proj/sub"}`)); err != nil {
		t.Fatal(err)
	}

	// Finish the write worker; the read worker keeps running.
	l.procs["write task"].tokens = &workers.Tokens{Input: 12, Output: 3, Total: 15}
	l.procs["write task"].finish("the write worker changed files on branch ds/w002", nil)
	waitWorkers(t, d, "the write worker to finish", func(b *workersBlock) bool {
		if b == nil {
			return false
		}
		for _, w := range b.Shown {
			if w.ID == "w002" && w.State == "done" {
				return true
			}
		}
		return false
	})

	b := statusWorkers(t, d)
	if b == nil {
		t.Fatal("status carried no workers block")
	}
	if b.Active != 1 {
		t.Fatalf("active = %d, want 1 (the read worker still runs)", b.Active)
	}
	byID := map[string]workerLine{}
	for _, w := range b.Shown {
		byID[w.ID] = w
	}
	read, ok := byID["w001"]
	if !ok || read.State != "running" || read.Agent != "codex" || read.Harness != "codex" || read.Mode != "read" {
		t.Fatalf("read worker view = %+v", read)
	}
	if read.Tokens != nil || read.Summary != "" {
		t.Fatalf("a running worker must carry no tokens or summary: %+v", read)
	}
	done, ok := byID["w002"]
	if !ok || done.State != "done" || done.Agent != "claude" || done.Harness != "claude-code" || done.Mode != "write" {
		t.Fatalf("write worker view = %+v", done)
	}
	if done.Tokens == nil || done.Tokens.Total != 15 {
		t.Fatalf("finished tokens = %+v", done.Tokens)
	}
	if !strings.Contains(done.Summary, "the write worker changed files") {
		t.Fatalf("finished summary = %q", done.Summary)
	}

	// No model is cravado: a plain status result must not carry a model id.
	env, _ := call(t, d, "status", `{}`)
	if strings.Contains(string(env["data"]), `"model"`) {
		t.Fatalf("status must not carry a model name: %s", env["data"])
	}
	d.Work.Close()
}

// Unhappy 1: no workers. The block is present but empty, with no error, so the
// orchestrator sees "nothing running" rather than a failure.
func TestStatusWithNoWorkersIsEmptyNotError(t *testing.T) {
	d := fullDeps()
	d.Work = workers.New(newViewLauncher(), 4, time.Now)
	b := statusWorkers(t, d)
	if b == nil {
		t.Fatal("the workers block must be present even with no workers")
	}
	if b.Active != 0 || len(b.Shown) != 0 || b.Omitted != 0 {
		t.Fatalf("empty workers block = %+v", b)
	}
	// Quota is unchanged: the status still reports server, version and playbooks.
	var s statusData
	env, _ := call(t, d, "status", `{}`)
	_ = json.Unmarshal(env["data"], &s)
	if s.Server != "downshift" || s.Playbooks != 2 {
		t.Fatalf("quota part changed: %+v", s)
	}
}

// Unhappy 2: workers are not wired on this server (nil queue). The status omits
// the workers block entirely and still answers with the quota part.
func TestStatusWithoutAQueueOmitsTheBlock(t *testing.T) {
	d := fullDeps() // no Work
	b := statusWorkers(t, d)
	if b != nil {
		t.Fatalf("a server without a queue must omit the workers block, got %+v", b)
	}
	if _, err := call(t, d, "status", `{}`); err != nil {
		t.Fatalf("status without a queue must still answer: %v", err)
	}
}

// Unhappy 3: many finished workers. Only the most recent are shown, newest
// first, and the rest are counted as omitted (none is silently dropped).
func TestStatusCapsFinishedWorkersAndCountsOmitted(t *testing.T) {
	l := newViewLauncher()
	d := fullDeps()
	d.Work = workers.New(l, 16, time.Now)
	d.Root = "/proj"

	const n = maxWorkersShown + 5
	tasks := make([]string, n)
	for i := range tasks {
		tasks[i] = "t" + string(rune('a'+i))
	}
	// delegate caps a call at maxTasksPerCall, so fan out in batches; the queue
	// numbers the jobs in the order they arrive across calls (w001..wNNN).
	for i := 0; i < n; i += maxTasksPerCall {
		end := min(i+maxTasksPerCall, n)
		payload, _ := json.Marshal(map[string]any{"tasks": tasks[i:end], "agent": "codex", "mode": "read"})
		if _, err := d.delegate(context.Background(), payload); err != nil {
			t.Fatal(err)
		}
	}
	for _, task := range tasks {
		l.procs[task].finish("done "+task, nil)
	}
	waitWorkers(t, d, "every worker to finish", func(b *workersBlock) bool {
		return b != nil && b.Active == 0
	})

	b := statusWorkers(t, d)
	if len(b.Shown) != maxWorkersShown {
		t.Fatalf("shown = %d, want the cap %d", len(b.Shown), maxWorkersShown)
	}
	if b.Omitted != n-maxWorkersShown {
		t.Fatalf("omitted = %d, want %d", b.Omitted, n-maxWorkersShown)
	}
	// Newest first: the last task delegated (w%03d highest) comes first.
	if b.Shown[0].ID != workerIDForTest(n) {
		t.Fatalf("first shown = %s, want the newest %s", b.Shown[0].ID, workerIDForTest(n))
	}
	d.Work.Close()
}

// workerIDForTest mirrors the queue's id format for the assertion above.
func workerIDForTest(n int) string {
	return "w" + leftPad(n)
}

func leftPad(n int) string {
	s := ""
	for _, d := range []int{100, 10, 1} {
		s += string(rune('0' + (n/d)%10))
	}
	return s
}
