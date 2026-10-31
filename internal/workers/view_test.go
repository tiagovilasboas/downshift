// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// movingNow is a clock the test advances by hand, so an elapsed time is exact
// and does not depend on how fast the test machine runs.
type movingNow struct{ t time.Time }

func (m *movingNow) now() time.Time { return m.t }
func (m *movingNow) add(d time.Duration) *movingNow {
	m.t = m.t.Add(d)
	return m
}

// Workers with no jobs is an empty list, not an error: the status tool asks on
// every call, including before anything was delegated.
func TestWorkersWithNoJobsIsEmpty(t *testing.T) {
	q := New(newFakeLauncher(), 2, fixedNow)
	if got := q.Workers(); len(got) != 0 {
		t.Fatalf("Workers on an empty queue = %+v, want none", got)
	}
}

// A running worker is reported with its elapsed time, its write flag and no
// tokens yet; a finished one keeps its elapsed frozen at the end, its tokens
// and a short summary of its report. State names match the delivery outcomes.
func TestWorkersReportsRunningAndFinished(t *testing.T) {
	l := newFakeLauncher()
	clk := &movingNow{t: fixedNow()}
	q := New(l, 2, clk.now)
	specs := []Spec{
		{Agent: "claude", Dir: ".", Task: "write one", Budget: time.Minute, Write: true},
		{Agent: "codex", Dir: ".", Task: "read one", Budget: time.Minute},
	}
	if _, err := q.Delegate(specs); err != nil {
		t.Fatal(err)
	}
	// Both are running at start; advance the clock and finish the write one.
	clk.add(3 * time.Second)
	l.proc("write one").tokens = &Tokens{Input: 10, Output: 5, Total: 15}
	l.proc("write one").finish("the write worker did the thing and more text here", nil)
	c := q.Collect(context.Background(), 0, false, 0)
	_ = c
	waitFor(t, "the write job to finish", func() bool {
		for _, w := range q.Workers() {
			if w.ID == "w001" && w.State == Done {
				return true
			}
		}
		return false
	})
	clk.add(2 * time.Second) // the running one keeps ticking; the finished one does not

	byID := map[string]WorkerView{}
	for _, w := range q.Workers() {
		byID[w.ID] = w
	}
	if len(byID) != 2 {
		t.Fatalf("want two views, got %d", len(byID))
	}

	done := byID["w001"]
	if done.State != Done || done.Agent != "claude" || done.Harness != "claude-code" {
		t.Fatalf("finished view = %+v", done)
	}
	if !done.Write {
		t.Fatalf("the write worker must report write=true: %+v", done)
	}
	if done.Elapsed != 3*time.Second {
		t.Fatalf("finished elapsed = %s, want frozen at 3s", done.Elapsed)
	}
	if done.Tokens == nil || done.Tokens.Total != 15 {
		t.Fatalf("finished tokens = %+v, want total 15", done.Tokens)
	}
	if done.Summary == "" || !strings.Contains(done.Summary, "the write worker") {
		t.Fatalf("finished summary = %q", done.Summary)
	}

	run := byID["w002"]
	if run.State != Running || run.Write {
		t.Fatalf("running view = %+v", run)
	}
	if run.Tokens != nil {
		t.Fatalf("a running worker reports no tokens yet: %+v", run.Tokens)
	}
	if run.Elapsed != 5*time.Second {
		t.Fatalf("running elapsed = %s, want 5s (still ticking)", run.Elapsed)
	}
}

// A failed worker's summary carries the start of its error, trimmed to the
// small cap, so a huge stderr dump never rides in the status view (R21).
func TestWorkersSummaryIsTrimmedSmall(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("bad", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	huge := strings.Repeat("x", 10_000)
	l.proc("bad").finish("", errors.New(huge))
	waitFor(t, "the job to fail", func() bool {
		for _, w := range q.Workers() {
			if w.State == Failed {
				return true
			}
		}
		return false
	})
	w := q.Workers()[0]
	if w.State != Failed {
		t.Fatalf("state = %s, want failed", w.State)
	}
	if len(w.Summary) > maxSummary+40 {
		t.Fatalf("summary is %d bytes, must be trimmed near %d", len(w.Summary), maxSummary)
	}
}

// Workers holds the lock only briefly: it must return while a collect is
// parked waiting for an active worker, so a status call never serializes behind
// a waiting collect (spec R22).
func TestWorkersAnswersWhileCollectWaits(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("slow", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	parked := make(chan struct{})
	go func() {
		q.Collect(context.Background(), 0, true, 5*time.Second)
		close(parked)
	}()
	// The collect is waiting (the worker runs for a minute). Workers must answer.
	done := make(chan []WorkerView, 1)
	go func() { done <- q.Workers() }()
	select {
	case views := <-done:
		if len(views) != 1 || views[0].State != Running {
			t.Fatalf("views while collect waits = %+v", views)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Workers blocked behind a waiting collect")
	}
	// Clean up: finishing the worker releases the parked collect.
	l.proc("slow").finish("ok", nil)
	select {
	case <-parked:
	case <-time.After(5 * time.Second):
		t.Fatal("collect did not return after the worker finished")
	}
}
