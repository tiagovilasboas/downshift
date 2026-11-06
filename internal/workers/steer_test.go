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

// Steer corrects a worker mid-turn without losing its conversation: it stops a
// live turn with the same kill cancel uses but keeps the job and its session,
// and runs the correction first. These tests prove that with the launch shim,
// never a real CLI. They are unhappy-path first (regra 1:3): the refusals and
// the no-session loss come before the happy interrupt-and-resume.

// --- unhappy: the refusals (Steer returns a clear error, changes nothing) ---

func TestSteerRefusesAnUnknownId(t *testing.T) {
	q := New(newResumeLaunch(), 1, fixedNow)
	if _, err := q.Steer("w404", "fix"); !errors.Is(err, ErrUnknownJob) {
		t.Fatalf("err = %v, want ErrUnknownJob", err)
	}
}

func TestSteerRefusesACancelledWorker(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	ids, err := q.Delegate([]Spec{continueSpec("run", time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	q.Cancel(ids)
	if _, err := q.Steer(ids[0], "fix"); !errors.Is(err, ErrNotSteerable) {
		t.Fatalf("err = %v, want ErrNotSteerable for a cancelled worker", err)
	}
	if calls := l.resumeCalls(); len(calls) != 0 {
		t.Fatalf("a cancelled worker was resumed: %+v", calls)
	}
}

func TestSteerRefusesAFailedWorker(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("bad", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("bad").finish("", errors.New("exit status 1"))
	waitFor(t, "the worker to fail", func() bool { return q.Statuses()[0].State == Failed })
	if _, err := q.Steer("w001", "fix"); !errors.Is(err, ErrNotSteerable) {
		t.Fatalf("err = %v, want ErrNotSteerable for a failed worker", err)
	}
}

func TestSteerRefusesASettledWorkerWithNoSessionId(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("ok", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	p := l.proc("ok")
	p.session = "" // the CLI wrote no session id, so the job is not resumable
	p.finish("done", nil)
	waitFor(t, "the worker to finish", func() bool { return q.Statuses()[0].State == Done })
	if _, err := q.Steer("w001", "fix"); !errors.Is(err, ErrNotSteerable) {
		t.Fatalf("err = %v, want ErrNotSteerable when no session was captured", err)
	}
}

func TestSteerRefusesAnEmptyMessage(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Steer("w001", "   "); !errors.Is(err, ErrEmptyTask) {
		t.Fatalf("err = %v, want ErrEmptyTask", err)
	}
}

func TestSteerRefusesAMessageOverTheLimit(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("x", MaxTaskBytes+1)
	if _, err := q.Steer("w001", big); !errors.Is(err, ErrTaskTooLong) {
		t.Fatalf("err = %v, want ErrTaskTooLong", err)
	}
}

func TestSteerRefusesWhenTheInboxIsFull(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("run", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the worker to run", func() bool { return q.Statuses()[0].State == Running })
	for i := 0; i < MaxInbox; i++ {
		if err := q.Continue("w001", "turn"); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}
	if _, err := q.Steer("w001", "one too many"); !errors.Is(err, ErrInboxFull) {
		t.Fatalf("err = %v, want ErrInboxFull", err)
	}
}

// TestSteerOnALiveTurnWithNoSessionDropsTheCorrection proves the loss the plan
// names: a live turn interrupted before it captured a session id cannot resume,
// so the correction is dropped with the worker and its delivery says so. The
// interrupted turn is the only delivery, and it is not resumable afterwards.
func TestSteerOnALiveTurnWithNoSessionDropsTheCorrection(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })
	p := l.proc("turn-1")
	p.session = "" // killed before the CLI wrote a session id
	p.report = "half-done"
	if _, err := q.Steer("w001", "actually do Y"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	// The interrupted turn settles; with no session, the correction cannot run.
	waitFor(t, "the interrupted turn to settle", func() bool { return q.Statuses()[0].State.Finished() })
	if calls := l.resumeCalls(); len(calls) != 0 {
		t.Fatalf("a worker with no session was resumed: %+v", calls)
	}
	c := q.Collect(context.Background(), 0, true, 5*time.Second)
	if len(c.Deliveries) != 1 || !strings.Contains(c.Deliveries[0].Text, "could not be applied") {
		t.Fatalf("delivery = %+v, want one that says the correction was dropped", c.Deliveries)
	}
	// The worker cannot be steered again: it is settled with no session.
	if _, err := q.Steer("w001", "again"); !errors.Is(err, ErrNotSteerable) {
		t.Fatalf("err = %v, want ErrNotSteerable after the drop", err)
	}
}

// --- happy: interrupt a live turn, degrade to a front send when idle ---

// TestSteerInterruptsALiveTurnAndRunsTheCorrectionFirst is the core path: a
// live turn is interrupted (killed, kept resumable), and the correction runs as
// the first turn of the resumed session, before any turn that was already
// queued. It proves the interrupt kills the process, the session survives, and
// the resume carries the correction with the original spec.
func TestSteerInterruptsALiveTurnAndRunsTheCorrectionFirst(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	first := continueSpec("turn-1", time.Minute)
	if _, err := q.Delegate([]Spec{first}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })
	// A turn was already queued behind the live one; the steer must run first.
	if err := q.Continue("w001", "queued-before-steer"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	p := l.proc("turn-1")

	if _, err := q.Steer("w001", "steer-correction"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	// The live turn was killed by the interrupt.
	select {
	case <-p.killed:
	case <-time.After(2 * time.Second):
		t.Fatal("steer did not interrupt the live turn")
	}
	// The interrupted turn settles resumable, then the correction resumes first.
	waitFor(t, "the correction to resume", func() bool { return len(l.resumeCalls()) == 1 })
	calls := l.resumeCalls()
	if calls[0].session != "turn-1" {
		t.Errorf("resumed with session %q, want turn-1 (the interrupted turn's id)", calls[0].session)
	}
	if calls[0].spec.Task != "steer-correction" {
		t.Errorf("first resumed task = %q, want steer-correction (ahead of the queued turn)", calls[0].spec.Task)
	}
	// Isolation held: same dir/model/effort/mode as the original spec.
	got := calls[0].spec
	if got.Dir != first.Dir || got.Model != first.Model || got.Effort != first.Effort || got.Write != first.Write {
		t.Errorf("steer changed the spec: got %+v, want the original %+v", got, first)
	}
	// Then the turn that was queued before the steer runs after the correction.
	l.proc("steer-correction").finish("corrected", nil)
	waitFor(t, "the pre-steer queued turn to resume", func() bool { return len(l.resumeCalls()) == 2 })
	if l.resumeCalls()[1].spec.Task != "queued-before-steer" {
		t.Errorf("second resumed task = %q, want queued-before-steer", l.resumeCalls()[1].spec.Task)
	}
}

// TestSteerOnASettledWorkerResumesWithTheCorrectionFirst proves the idle
// degrade for a settled worker: with no live turn, steer is a front-of-inbox
// send that resumes the session now with the correction.
func TestSteerOnASettledWorkerResumesWithTheCorrectionFirst(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	l.proc("turn-1").finish("first", nil)
	q.Collect(context.Background(), 0, true, 5*time.Second)

	if _, err := q.Steer("w001", "correction"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	waitFor(t, "the correction to resume", func() bool { return len(l.resumeCalls()) == 1 })
	if l.resumeCalls()[0].spec.Task != "correction" || l.resumeCalls()[0].session != "turn-1" {
		t.Fatalf("resume = %+v, want the correction on session turn-1", l.resumeCalls()[0])
	}
}

// TestSteerOnAQueuedWorkerJumpsTheQueue proves steer on a worker that has not
// started a turn yet is a front-of-inbox send: it does not interrupt (there is
// nothing live) and its correction runs before any later send when the worker
// settles its first turn.
func TestSteerOnAQueuedWorkerJumpsTheQueue(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	// Two jobs, limit 1: the second stays queued while the first runs.
	if _, err := q.Delegate([]Spec{continueSpec("busy", time.Minute), continueSpec("waiting", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the first job to run", func() bool { return q.Statuses()[0].State == Running })
	if q.Statuses()[1].State != Queued {
		t.Fatalf("second job state = %s, want queued", q.Statuses()[1].State)
	}
	// A send then a steer on the queued worker: the steer must sit ahead.
	if err := q.Continue("w002", "later"); err != nil {
		t.Fatalf("send on queued: %v", err)
	}
	if _, err := q.Steer("w002", "first"); err != nil {
		t.Fatalf("steer on queued: %v", err)
	}
	// A queued worker's steer does not resume anything yet.
	if calls := l.resumeCalls(); len(calls) != 0 {
		t.Fatalf("steer on a queued worker resumed early: %+v", calls)
	}
	// Free the slot so the queued worker starts its first turn and drains.
	l.proc("busy").finish("done", nil)
	waitFor(t, "the queued worker to start", func() bool { return q.Statuses()[1].State == Running })
	l.proc("waiting").finish("ran", nil)
	// The inbox drains the steer first, then the earlier send.
	waitFor(t, "the steer to resume first", func() bool { return len(l.resumeCalls()) == 1 })
	if l.resumeCalls()[0].spec.Task != "first" {
		t.Fatalf("first drained task = %q, want first (steer ahead of the send)", l.resumeCalls()[0].spec.Task)
	}
}

// TestCancelAfterSteerKillsTheResumedTurnAndDropsTheInbox proves cancel still
// wins after a steer: once the correction is resuming, a cancel kills it and the
// worker ends cancelled with no further turns.
func TestCancelAfterSteerKillsTheResumedTurnAndDropsTheInbox(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })
	if _, err := q.Steer("w001", "correction"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	waitFor(t, "the correction to resume", func() bool { return len(l.resumeCalls()) == 1 })
	waitFor(t, "the resumed turn to run", func() bool { return q.Statuses()[0].State == Running })

	acted := q.Cancel([]string{"w001"})
	if len(acted) != 1 {
		t.Fatalf("cancel acted on %v, want w001", acted)
	}
	select {
	case <-l.proc("correction").killed:
	case <-time.After(2 * time.Second):
		t.Fatal("cancel after steer did not kill the resumed turn")
	}
	if q.Statuses()[0].State != Cancelled {
		t.Fatalf("state = %s after cancel, want cancelled", q.Statuses()[0].State)
	}
}

// TestSteerDoesNotBlockCollectCancelStatus is the concurrency guarantee (regra
// 2): while a steer interrupts and the session resumes, collect, cancel and
// status all answer without serialising behind the turn.
func TestSteerDoesNotBlockCollectCancelStatus(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })
	if _, err := q.Steer("w001", "correction"); err != nil {
		t.Fatalf("steer: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = q.Workers()
		_ = q.Statuses()
		_ = q.Collect(context.Background(), 0, false, 0)
		_ = q.Cancel([]string{"w001"})
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("collect/cancel/status blocked behind a steer")
	}
}

// TestSteerReapsTheInterruptedProcess proves no process leak: the interrupted
// turn's process is waited on (reaped) by the queue's wait goroutine, which is
// what settles the job and resumes the correction. If the process were not
// reaped, the job would never settle and the correction would never resume, so
// a resumed correction is itself the evidence the interrupted process was reaped.
func TestSteerReapsTheInterruptedProcess(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	if _, err := q.Delegate([]Spec{continueSpec("turn-1", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "turn 1 to run", func() bool { return q.Statuses()[0].State == Running })
	p := l.proc("turn-1")
	if _, err := q.Steer("w001", "correction"); err != nil {
		t.Fatalf("steer: %v", err)
	}
	// The interrupted process was killed (reaped by Wait returning its error).
	select {
	case <-p.killed:
	case <-time.After(2 * time.Second):
		t.Fatal("the interrupted process was not killed")
	}
	// The resume only happens from wait, after the killed process is reaped, so
	// a resumed correction proves the reap happened and the slot was reused.
	waitFor(t, "the correction to resume after the reap", func() bool { return len(l.resumeCalls()) == 1 })
}
