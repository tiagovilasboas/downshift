// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// fakeProc is a worker the test ends by hand.
type fakeProc struct {
	report string
	stderr string
	done   chan error
	killed chan struct{}
	once   sync.Once
}

func newFakeProc() *fakeProc {
	return &fakeProc{done: make(chan error, 1), killed: make(chan struct{})}
}

func (p *fakeProc) Wait() error {
	select {
	case err := <-p.done:
		return err
	case <-p.killed:
		return errors.New("killed")
	}
}

func (p *fakeProc) Kill() error {
	p.once.Do(func() { close(p.killed) })
	return nil
}

func (p *fakeProc) Report() string { return p.report }
func (p *fakeProc) Stderr() string { return p.stderr }

// finish ends the worker with the given report and error.
func (p *fakeProc) finish(report string, err error) {
	p.report = report
	p.done <- err
}

// fakeLauncher hands out fake workers and remembers which tasks it started.
type fakeLauncher struct {
	mu       sync.Mutex
	procs    map[string]*fakeProc
	launched []string
	fail     error
}

func newFakeLauncher() *fakeLauncher {
	return &fakeLauncher{procs: map[string]*fakeProc{}}
}

func (l *fakeLauncher) Launch(s Spec) (Process, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.fail != nil {
		return nil, l.fail
	}
	p := newFakeProc()
	l.procs[s.Task] = p
	l.launched = append(l.launched, s.Task)
	return p, nil
}

func (l *fakeLauncher) proc(task string) *fakeProc {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.procs[task]
}

func (l *fakeLauncher) started() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.launched...)
}

var fixedNow = func() time.Time { return time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC) }

func spec(task string, budget time.Duration) Spec {
	return Spec{Agent: "claude", Dir: ".", Task: task, Budget: budget}
}

// waitFor polls until cond holds or the test fails.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestDelegateRefusesAJobWithoutABudget(t *testing.T) {
	q := New(newFakeLauncher(), 2, fixedNow)
	if _, err := q.Delegate([]Spec{spec("a", 0)}); !errors.Is(err, ErrNoBudget) {
		t.Fatalf("err = %v, want ErrNoBudget", err)
	}
}

func TestLimitRunsOneWorkerAtATime(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	ids, err := q.Delegate([]Spec{spec("first", time.Minute), spec("second", time.Minute)})
	if err != nil || len(ids) != 2 {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
	if got := l.started(); len(got) != 1 || got[0] != "first" {
		t.Fatalf("started = %v, want only first", got)
	}
	l.proc("first").finish("first report", nil)
	waitFor(t, "second start", func() bool { return len(l.started()) == 2 })
}

func TestCollectReturnsDeliveriesUntilAcknowledged(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("only", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("only").finish("the answer", nil)
	ctx := context.Background()

	c := q.Collect(ctx, 0, true, 5*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "done" || c.Deliveries[0].Text != "the answer" || c.Deliveries[0].Repeat {
		t.Fatalf("first collect = %+v", c.Deliveries)
	}
	if c.Ack == 0 || c.Active != 0 {
		t.Fatalf("ack = %d, active = %d", c.Ack, c.Active)
	}

	again := q.Collect(ctx, 0, false, 0)
	if len(again.Deliveries) != 1 || !again.Deliveries[0].Repeat {
		t.Fatalf("unacknowledged delivery must come back as a repeat: %+v", again.Deliveries)
	}

	acked := q.Collect(ctx, c.Ack, false, 0)
	if len(acked.Deliveries) != 0 {
		t.Fatalf("acknowledged delivery came back: %+v", acked.Deliveries)
	}
}

func TestSequenceStartsFromTheProcessStart(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("x", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("x").finish("ok", nil)
	c := q.Collect(context.Background(), 0, true, 5*time.Second)
	if c.Deliveries[0].Seq <= uint64(fixedNow().UnixMilli()) {
		t.Fatalf("seq %d must be above the start time in milliseconds", c.Deliveries[0].Seq)
	}
}

func TestCancelStopsRunningAndQueuedJobs(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	ids, err := q.Delegate([]Spec{spec("running", time.Minute), spec("queued", time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	acted := q.Cancel(ids)
	if len(acted) != 2 {
		t.Fatalf("acted = %v, want both", acted)
	}
	if got := l.started(); len(got) != 1 {
		t.Fatalf("a cancelled queued job was started: %v", got)
	}
	c := q.Collect(context.Background(), 0, false, 0)
	if len(c.Deliveries) != 2 || c.Deliveries[0].Outcome != "cancelled" || c.Deliveries[1].Outcome != "cancelled" {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
	if again := q.Cancel(ids); len(again) != 0 {
		t.Fatalf("cancelling finished jobs must act on none, got %v", again)
	}
}

func TestTimeBudgetEndsAJobWithItsPartialOutput(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("slow", 30*time.Millisecond)}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := q.Collect(ctx, 0, true, 5*time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "timeout" || !strings.Contains(c.Deliveries[0].Text, "time budget") {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
}

func TestStartFailureIsADeliveryNotAHang(t *testing.T) {
	l := newFakeLauncher()
	l.fail = ErrNoVerifiedMode
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("x", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "failed" || !strings.Contains(c.Deliveries[0].Text, "start failed") {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
}

func TestAFailedWorkerReportsItsStderrTail(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("bad", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	p := l.proc("bad")
	p.stderr = "quota exhausted"
	p.finish("", errors.New("exit status 1"))
	c := q.Collect(context.Background(), 0, true, time.Second)
	if c.Deliveries[0].Outcome != "failed" || !strings.Contains(c.Deliveries[0].Text, "quota exhausted") {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
}

func TestCollectWaitsUntilNothingIsActive(t *testing.T) {
	l := newFakeLauncher()
	q := New(l, 2, fixedNow)
	if _, err := q.Delegate([]Spec{spec("a", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	got := make(chan Collection, 1)
	go func() { got <- q.Collect(context.Background(), 0, true, 5*time.Second) }()
	select {
	case <-got:
		t.Fatal("collect returned while a worker was still running")
	case <-time.After(50 * time.Millisecond):
	}
	l.proc("a").finish("done", nil)
	select {
	case c := <-got:
		if c.Active != 0 || len(c.Deliveries) != 1 {
			t.Fatalf("collection = %+v", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("collect did not return after the worker finished")
	}
}

func TestUnverifiedLauncherNeverStartsAWorker(t *testing.T) {
	q := New(Unverified(), 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("x", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	c := q.Collect(context.Background(), 0, true, time.Second)
	if len(c.Deliveries) != 1 || c.Deliveries[0].Outcome != "failed" || !strings.Contains(c.Deliveries[0].Text, "R27") {
		t.Fatalf("deliveries = %+v", c.Deliveries)
	}
}

func TestTailKeepsOnlyTheEnd(t *testing.T) {
	tl := NewTail(5)
	_, _ = tl.Write([]byte("abc"))
	_, _ = tl.Write([]byte("defgh"))
	if got := tl.String(); got != "defgh" {
		t.Fatalf("tail = %q, want defgh", got)
	}
}

func TestCapTextCutsOnACharacterBoundary(t *testing.T) {
	long := strings.Repeat("é", maxOutput)
	got := capText(long)
	if !utf8.ValidString(got) || !strings.Contains(got, "[cut") || len(got) > maxOutput+80 {
		t.Fatalf("cut text is not valid or not bounded: %d bytes", len(got))
	}
	if short := capText("short"); short != "short" {
		t.Fatalf("short text changed: %q", short)
	}
}
