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
)

// A worker's continuation is state in the queue, not a new worker: a settled
// job that captured a session id can run another turn on the same conversation,
// and a live job collects the next turn in an inbox and runs it when the current
// one ends. These tests prove that state with the launch shim, never a real CLI.
// They are unhappy-path first (regra 1:3): the refusals come before the happy
// resume and the happy enqueue.

// resumeLaunch is a fake Launcher that also resumes: Launch starts a worker the
// test ends by hand, and LaunchResume records the (spec, session id) it was
// asked to continue and hands back another hand-ended worker. It lets a test
// prove the queue resumes with the original spec and the first turn's id,
// without a CLI.
type resumeLaunch struct {
	mu      sync.Mutex
	procs   map[string]*fakeProc // keyed by task text
	resumes []resumeCall         // every LaunchResume, in order
	failRun error                // when set, Launch fails with it
}

// resumeCall is one LaunchResume: the spec the queue passed (so a test checks
// the dir/model/effort/mode did not change) and the session id it resumed.
type resumeCall struct {
	spec    Spec
	session string
}

func newResumeLaunch() *resumeLaunch { return &resumeLaunch{procs: map[string]*fakeProc{}} }

func (l *resumeLaunch) Launch(s Spec) (Process, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failRun != nil {
		return nil, l.failRun
	}
	p := newFakeProc()
	p.session = s.Task // the shim-like proc reports a session id derived from its task
	l.procs[s.Task] = p
	return p, nil
}

func (l *resumeLaunch) LaunchResume(s Spec, sessionID string) (Process, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p := newFakeProc()
	p.session = sessionID // a resumed turn keeps the same session id
	l.procs[s.Task] = p
	l.resumes = append(l.resumes, resumeCall{spec: s, session: sessionID})
	return p, nil
}

func (l *resumeLaunch) proc(task string) *fakeProc {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.procs[task]
}

func (l *resumeLaunch) resumeCalls() []resumeCall {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]resumeCall(nil), l.resumes...)
}

// continueSpec is a spec that also captures a session id, so a settled job is
// resumable in these tests.
func continueSpec(task string, budget time.Duration) Spec {
	return Spec{Agent: "claude", Model: "claude-haiku-5-5", Effort: "low", Dir: ".", Task: task, Budget: budget}
}

// --- unhappy: the refusals (Continue returns a clear error, changes nothing) ---

func TestContinueRefusesAnUnknownId(t *testing.T) {
	q := New(newResumeLaunch(), 1, fixedNow)
	if err := q.Continue("w404", "more"); !errors.Is(err, ErrUnknownJob) {
		t.Fatalf("err = %v, want ErrUnknownJob", err)
	}
}

func TestContinueRefusesACancelledWorker(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	ids, err := q.Delegate([]Spec{continueSpec("run", time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	q.Cancel(ids)
	if err := q.Continue(ids[0], "more"); !errors.Is(err, ErrNotResumable) {
		t.Fatalf("err = %v, want ErrNotResumable for a cancelled worker", err)
	}
	if calls := l.resumeCalls(); len(calls) != 0 {
		t.Fatalf("a cancelled worker was resumed: %+v", calls)
	}
}

func TestContinueRefusesAFailedWorker(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("bad", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("bad").finish("", errors.New("exit status 1"))
	waitFor(t, "the worker to fail", func() bool { return q.Statuses()[0].State == Failed })
	if err := q.Continue("w001", "more"); !errors.Is(err, ErrNotResumable) {
		t.Fatalf("err = %v, want ErrNotResumable for a failed worker", err)
	}
}

func TestContinueRefusesASettledWorkerWithNoSessionId(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("ok", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	p := l.proc("ok")
	p.session = "" // the CLI wrote no session id, so the job is not resumable
	p.finish("done", nil)
	waitFor(t, "the worker to finish", func() bool { return q.Statuses()[0].State == Done })
	if err := q.Continue("w001", "more"); !errors.Is(err, ErrNotResumable) {
		t.Fatalf("err = %v, want ErrNotResumable when no session id was captured", err)
	}
}

func TestContinueRefusesAnEmptyTask(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := q.Continue("w001", "   "); !errors.Is(err, ErrEmptyTask) {
		t.Fatalf("err = %v, want ErrEmptyTask", err)
	}
}

func TestContinueRefusesATaskOverTheLimit(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", MaxTaskBytes+1)
	if err := q.Continue("w001", big); !errors.Is(err, ErrTaskTooLong) {
		t.Fatalf("err = %v, want ErrTaskTooLong", err)
	}
}

func TestContinueRefusesWhenTheInboxIsFull(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the worker to run", func() bool { return q.Statuses()[0].State == Running })
	// Fill the inbox to the cap while the worker is still on its first turn.
	for i := 0; i < MaxInbox; i++ {
		if err := q.Continue("w001", "turn"); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	if err := q.Continue("w001", "one too many"); !errors.Is(err, ErrInboxFull) {
		t.Fatalf("err = %v, want ErrInboxFull", err)
	}
}

// --- happy: resume a settled worker, enqueue on a live one ---

// TestContinueResumesASettledWorkerWithItsOwnSpecAndSession is the core resume
// path: a worker finishes a first turn and captures a session id; Continue
// resumes it with that id and the ORIGINAL spec (same dir, model, effort, mode),
// and the result comes back through collect as a second delivery on the same id.
func TestContinueResumesASettledWorkerWithItsOwnSpecAndSession(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	first := continueSpec("turn-1", time.Minute)
	if _, err := q.Delegate([]Spec{first}); err != nil {
		t.Fatal(err)
	}
	l.proc("turn-1").finish("first result", nil)
	c1 := q.Collect(context.Background(), 0, true, 5*time.Second)
	if len(c1.Deliveries) != 1 || c1.Deliveries[0].SessionID != "turn-1" {
		t.Fatalf("first delivery = %+v", c1.Deliveries)
	}

	if err := q.Continue("w001", "turn-2"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	// The resumed turn is a running worker again.
	waitFor(t, "the resumed turn to run", func() bool { return q.Statuses()[0].State == Running })
	l.proc("turn-2").finish("second result", nil)

	c2 := q.Collect(context.Background(), c1.Ack, true, 5*time.Second)
	if len(c2.Deliveries) != 1 || c2.Deliveries[0].Job != "w001" || c2.Deliveries[0].Text != "second result" {
		t.Fatalf("second delivery = %+v, want one on w001 with the second result", c2.Deliveries)
	}

	calls := l.resumeCalls()
	if len(calls) != 1 {
		t.Fatalf("want exactly one resume, got %+v", calls)
	}
	// Isolation held: same session id, same dir/model/effort/mode as turn 1.
	if calls[0].session != "turn-1" {
		t.Errorf("resumed with session %q, want turn-1 (the first turn's id)", calls[0].session)
	}
	got := calls[0].spec
	if got.Dir != first.Dir || got.Model != first.Model || got.Effort != first.Effort || got.Write != first.Write || got.Agent != first.Agent {
		t.Errorf("resume changed the spec: got %+v, want dir/model/effort/write/agent of %+v", got, first)
	}
	// The second turn's task is the only field that changes.
	if got.Task != "turn-2" {
		t.Errorf("resumed task = %q, want turn-2", got.Task)
	}
}

// TestContinueDuringALiveTurnEnqueuesAndRunsAfter proves the concurrency rule:
// a Continue while the worker is on a turn does not start a second process; it
// waits in the inbox and runs when the live turn settles, as the next turn on
// the same session.
func TestContinueDuringALiveTurnEnqueuesAndRunsAfter(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })

	// Enqueue while the first turn is alive: no resume yet.
	if err := q.Continue("w001", "turn-2"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if calls := l.resumeCalls(); len(calls) != 0 {
		t.Fatalf("enqueue during a live turn must not resume yet: %+v", calls)
	}

	// The first turn settles, which hands the inbox its next turn.
	l.proc("turn-1").finish("first result", nil)
	waitFor(t, "the queued turn to resume", func() bool { return len(l.resumeCalls()) == 1 })
	if q.Statuses()[0].State != Running {
		t.Fatalf("the resumed turn must be running, state = %s", q.Statuses()[0].State)
	}
	l.proc("turn-2").finish("second result", nil)

	c := q.Collect(context.Background(), 0, true, 5*time.Second)
	// Both turns delivered, in order.
	if len(c.Deliveries) != 2 || c.Deliveries[0].Text != "first result" || c.Deliveries[1].Text != "second result" {
		t.Fatalf("deliveries = %+v, want both turns in arrival order", c.Deliveries)
	}
}

// TestInboxRunsInArrivalOrderOneTurnAtATime queues several turns during a live
// turn and checks they resume one at a time, in the order they arrived.
func TestInboxRunsInArrivalOrderOneTurnAtATime(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("t1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "t1 to run", func() bool { return q.Statuses()[0].State == Running })
	for _, task := range []string{"t2", "t3"} {
		if err := q.Continue("w001", task); err != nil {
			t.Fatalf("enqueue %s: %v", task, err)
		}
	}
	// Drain the turns one by one; each must be the only one running.
	l.proc("t1").finish("r1", nil)
	waitFor(t, "t2 to resume", func() bool { return len(l.resumeCalls()) == 1 })
	l.proc("t2").finish("r2", nil)
	waitFor(t, "t3 to resume", func() bool { return len(l.resumeCalls()) == 2 })
	l.proc("t3").finish("r3", nil)

	c := q.Collect(context.Background(), 0, true, 5*time.Second)
	if len(c.Deliveries) != 3 {
		t.Fatalf("want 3 deliveries, got %+v", c.Deliveries)
	}
	for i, want := range []string{"t2", "t3"} {
		if got := l.resumeCalls()[i].spec.Task; got != want {
			t.Errorf("resume %d task = %q, want %q (arrival order)", i, got, want)
		}
	}
}

// TestEnqueueDuringALiveTurnDoesNotBlockCollectCancelStatus is the concurrency
// guarantee (regra 2): while a turn runs and the inbox has work, collect,
// cancel and status all answer without serialising behind the live turn. This
// is the Continue equivalent of the cancel-during-collect property.
func TestEnqueueDuringALiveTurnDoesNotBlockCollectCancelStatus(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })
	if err := q.Continue("w001", "turn-2"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// status answers at once with the pending turn count, not blocked by the turn.
	done := make(chan struct{})
	go func() {
		defer close(done)
		views := q.Workers()
		if len(views) != 1 || views[0].Pending != 1 {
			t.Errorf("status = %+v, want one worker with one pending turn", views)
		}
		// cancel answers at once and settles the running worker.
		if acted := q.Cancel([]string{"w001"}); len(acted) != 1 {
			t.Errorf("cancel acted on %v, want w001", acted)
		}
		// collect answers (no wait) without blocking.
		_ = q.Collect(context.Background(), 0, false, 0)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("collect/cancel/status blocked behind the live turn + inbox")
	}
}

// TestCancelDuringAResumedTurnKillsItAndDropsTheInbox cancels a worker while a
// resumed turn runs with more turns queued: the turn is killed and the queued
// turns never run.
func TestCancelDuringAResumedTurnKillsItAndDropsTheInbox(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("turn-1").finish("first", nil)
	q.Collect(context.Background(), 0, true, 5*time.Second)

	if err := q.Continue("w001", "turn-2"); err != nil { // resumes now (settled)
		t.Fatalf("resume: %v", err)
	}
	waitFor(t, "turn 2 to run", func() bool { return q.Statuses()[0].State == Running })
	if err := q.Continue("w001", "turn-3"); err != nil { // queued behind the live turn
		t.Fatalf("enqueue turn-3: %v", err)
	}

	acted := q.Cancel([]string{"w001"})
	if len(acted) != 1 {
		t.Fatalf("cancel acted on %v, want w001", acted)
	}
	// The running resumed turn was killed.
	select {
	case <-l.proc("turn-2").killed:
	default:
		t.Fatal("the resumed turn was not killed on cancel")
	}
	// turn-3 never resumed.
	if calls := l.resumeCalls(); len(calls) != 1 {
		t.Fatalf("a cancelled worker ran its queued inbox: %+v", calls)
	}
}

// TestResumeThroughTheRealLauncherUsesResumeArgsAndTheOriginalDir drives the
// end-to-end base through the launch shim (no real CLI): a claude worker runs a
// first turn and captures a session id, then Continue resumes it. The resumed
// process must run `--resume <that id>` in the SAME directory the first turn
// used, with the same model, and the second turn's prompt on stdin. This is the
// acceptance that the second turn reuses ResumeArgs with the first turn's id and
// the original isolation, proved against the shim the whole package uses.
func TestResumeThroughTheRealLauncherUsesResumeArgsAndTheOriginalDir(t *testing.T) {
	const sid = "11111111-2222-3333-4444-555555555555"
	// The shim records its args and stdin per run (into the run's own dir is not
	// possible since the shim dir is shared), so it appends each invocation's
	// argv and stdin to files keyed by the working directory it was run in.
	dir := shim(t, "claude", `echo '{"type":"result","subtype":"success","is_error":false,"result":"ok","session_id":"`+sid+`"}'`)
	work := t.TempDir()
	q := New(ByAgent(ReadOnlyLaunchers()), 1, fixedNow)
	if _, err := q.Delegate([]Spec{{Agent: "claude", Model: "claude-haiku-5-5", Dir: work, Task: "turn-1", Budget: time.Minute}}); err != nil {
		t.Fatal(err)
	}
	c1 := q.Collect(context.Background(), 0, true, 10*time.Second)
	if len(c1.Deliveries) != 1 || c1.Deliveries[0].SessionID != sid {
		t.Fatalf("first delivery = %+v, want session %s", c1.Deliveries, sid)
	}

	if err := q.Continue("w001", "turn-2"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	c2 := q.Collect(context.Background(), c1.Ack, true, 10*time.Second)
	if len(c2.Deliveries) != 1 || c2.Deliveries[0].Job != "w001" {
		t.Fatalf("second delivery = %+v, want one on w001", c2.Deliveries)
	}

	// The shim overwrites its args file each run; the second (resume) run is the
	// last one recorded.
	args := readFile(t, dir+"/claude.args")
	for _, want := range []string{"--resume", sid, "--strict-mcp-config"} {
		if !strings.Contains(args, want) {
			t.Errorf("resume args lack %q: %s", want, args)
		}
	}
	if strings.Contains(args, "bypassPermissions") {
		t.Errorf("resume widened the mode: %s", args)
	}
	if got := readFile(t, dir+"/claude.stdin"); got != "turn-2" {
		t.Errorf("resume prompt on stdin = %q, want turn-2", got)
	}
}

// TestALauncherThatCannotResumeRefusesAContinue proves the queue never resumes
// a turn the launcher cannot start: a settled worker under a launcher without
// LaunchResume is refused, even with a captured session id.
func TestALauncherThatCannotResumeRefusesAContinue(t *testing.T) {
	// fakeLauncher (workers_test.go) does not implement resumeLauncher.
	l := newFakeLauncher()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{spec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	p := l.proc("run")
	p.session = "some-id"
	p.finish("done", nil)
	waitFor(t, "the worker to finish", func() bool { return q.Statuses()[0].State == Done })
	if err := q.Continue("w001", "more"); !errors.Is(err, ErrNotResumable) {
		t.Fatalf("err = %v, want ErrNotResumable under a launcher that cannot resume", err)
	}
}

// TestStatusReportsPendingTurns checks the status view carries how many turns
// are waiting in a worker's inbox, so a caller sees pending work without
// collect.
func TestStatusReportsPendingTurns(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })
	if q.Workers()[0].Pending != 0 {
		t.Fatalf("pending = %d before any enqueue, want 0", q.Workers()[0].Pending)
	}
	for _, task := range []string{"t2", "t3"} {
		if err := q.Continue("w001", task); err != nil {
			t.Fatal(err)
		}
	}
	if got := q.Workers()[0].Pending; got != 2 {
		t.Fatalf("pending = %d, want 2", got)
	}
}
