// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"context"
	"errors"
	"testing"
	"time"
)

// A follow-up turn (send or steer) passes the same limits as a new worker: it
// waits for a slot under the concurrency limit, it is refused once the agent's
// token budget is spent, and the session caps how many follow-ups it runs.

// settledWorker delegates one job and ends it Done with a session id, so it is
// resumable. It returns the job id.
func settledWorker(t *testing.T, q *Queue, l *resumeLaunch, task string, tokens *Tokens) string {
	t.Helper()
	ids, err := q.Delegate([]Spec{continueSpec(task, time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	p := l.proc(task)
	p.tokens = tokens
	p.finish("done "+task, nil)
	waitFor(t, task+" to settle", func() bool { return stateOf(q, ids[0]) == Done })
	return ids[0]
}

func stateOf(q *Queue, id string) State {
	for _, s := range q.Statuses() {
		if s.ID == id {
			return s.State
		}
	}
	return ""
}

func runningCount(q *Queue) int {
	n := 0
	for _, s := range q.Statuses() {
		if s.State == Running {
			n++
		}
	}
	return n
}

func TestSendWaitsForASlotWithinTheLimit(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	a := settledWorker(t, q, l, "a", nil)
	if _, err := q.Delegate([]Spec{continueSpec("b", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if err := q.Continue(a, "more"); err != nil {
		t.Fatal(err)
	}
	if got := stateOf(q, a); got != Queued || runningCount(q) != 1 || len(l.resumeCalls()) != 0 {
		t.Fatalf("send ran past the limit: state %s, running %d, resumes %d", got, runningCount(q), len(l.resumeCalls()))
	}
	l.proc("b").finish("done b", nil)
	waitFor(t, "the follow-up to take the freed slot", func() bool { return stateOf(q, a) == Running })
	if calls := l.resumeCalls(); len(calls) != 1 || calls[0].session != "a" {
		t.Fatalf("resumes = %+v, want one resume of session a", calls)
	}
}

func TestSteerOnASettledWorkerWaitsForASlot(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	a := settledWorker(t, q, l, "a", nil)
	if _, err := q.Delegate([]Spec{continueSpec("b", time.Minute)}); err != nil {
		t.Fatal(err)
	}
	out, err := q.Steer(a, "fix")
	if err != nil {
		t.Fatal(err)
	}
	if out.Steering || out.Position != 1 || runningCount(q) != 1 {
		t.Fatalf("steer ran past the limit: %+v, running %d", out, runningCount(q))
	}
	l.proc("b").finish("done b", nil)
	waitFor(t, "the correction to run", func() bool { return stateOf(q, a) == Running })
}

func TestSendAndSteerRefuseOnceTheBudgetIsSpent(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 2, fixedNow)
	q.SetTokenBudget(100)
	a := settledWorker(t, q, l, "a", &Tokens{Total: 150})
	var budget ErrTokenBudget
	if err := q.Continue(a, "more"); !errors.As(err, &budget) {
		t.Fatalf("send err = %v, want ErrTokenBudget", err)
	}
	if _, err := q.Steer(a, "fix"); !errors.As(err, &budget) {
		t.Fatalf("steer err = %v, want ErrTokenBudget", err)
	}
	if calls := l.resumeCalls(); len(calls) != 0 {
		t.Fatalf("a turn ran past the spent budget: %+v", calls)
	}
}

// A turn queued while the budget was left is refused at start once the turn
// before it spent the budget: the job fails with the reason, nothing resumes.
func TestAQueuedTurnStopsWhenTheTurnBeforeSpentTheBudget(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	q.SetTokenBudget(100)
	ids, err := q.Delegate([]Spec{continueSpec("a", time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Continue(ids[0], "more"); err != nil {
		t.Fatal(err)
	}
	p := l.proc("a")
	p.tokens = &Tokens{Total: 150}
	p.finish("done a", nil)
	waitFor(t, "the queued turn to be refused", func() bool { return stateOf(q, ids[0]) == Failed })
	if calls := l.resumeCalls(); len(calls) != 0 {
		t.Fatalf("a turn ran past the spent budget: %+v", calls)
	}
}

func TestFollowUpTurnsStopAtTheSessionCap(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	q.SetMaxJobs(1)
	a := settledWorker(t, q, l, "a", nil)
	for i := 0; i < TurnsPerWorker; i++ {
		if err := q.Continue(a, "more"); err != nil {
			t.Fatalf("turn %d: %v", i+1, err)
		}
	}
	if err := q.Continue(a, "one too many"); !errors.Is(err, ErrTurnCapReached) {
		t.Fatalf("send err = %v, want ErrTurnCapReached", err)
	}
	if _, err := q.Steer(a, "fix"); !errors.Is(err, ErrTurnCapReached) {
		t.Fatalf("steer err = %v, want ErrTurnCapReached", err)
	}
}

// A timed-out turn with a queued follow-up resumes; the killed process of the
// timed-out turn exits afterwards, and that late exit must not settle the
// resumed turn as failed.
func TestALateExitOfATimedOutTurnDoesNotSettleTheNextTurn(t *testing.T) {
	l := newResumeLaunch()
	q := New(l, 1, fixedNow)
	ids, err := q.Delegate([]Spec{continueSpec("a", 100*time.Millisecond)})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.Continue(ids[0], "more"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := q.Collect(ctx, 0, true, 5*time.Second)
	if len(c.Deliveries) != 2 {
		t.Fatalf("deliveries = %+v, want the timed-out turn and its follow-up", c.Deliveries)
	}
	for i, d := range c.Deliveries {
		if d.Outcome != "timeout" {
			t.Errorf("turn %d outcome = %q, want timeout: %s", i+1, d.Outcome, d.Text)
		}
	}
}
