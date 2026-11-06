// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// Steer corrects a worker mid-turn without losing its conversation. It is the
// internal base the steer tool calls (task T4 of
// docs/internal/plans/mcp-worker-continuation.md); no MCP tool is wired here.
//
// The spike proved a second message on a live stream is not an interrupt: it is
// enqueued as the next turn (spike 2(b)). So there is no native mid-turn steer
// for the recipes Downshift runs; steer is built generically, the same way for
// every agent, out of two pieces the queue already owns:
//
//   - stop the live turn with the same kill cancel uses, but keep the job and
//     its session id so the conversation survives (the interrupt);
//   - put the correction at the FRONT of the inbox, before whatever was already
//     waiting, so when the session resumes the correction runs first.
//
// This never changes dir, model, effort or mode and never climbs a tier: the
// resume reuses the original spec exactly, like every other continuation. The
// Codex CLI also shows a native `turn/steer` in its help, but the spike ran no
// paid turn against it, so it is left as a future improvement rather than a path
// taken without proof.
//
// By worker state:
//
//   - Running (a live turn): interrupt it, prepend the correction, resume. The
//     interrupted turn settles as a resumable end, not a failure, so the session
//     is kept; the correction is the first turn of the resumed conversation.
//   - Queued (started but not yet on a turn) or Blocked (a live worker waiting):
//     there is no turn to interrupt, so steer is a front-of-inbox send: the
//     correction jumps ahead of any queued turn and runs first when the worker's
//     turn settles.
//   - Settled and resumable (done or timeout with a captured session): resume
//     now with the correction at the front of the inbox.
//   - Not steerable (unknown id, cancelled, failed, or settled with no captured
//     session): refused with a clear error, so the caller uses send/delegate.

// SteerOutcome is what Steer did, so the tool reports it without reading a racy
// post-state: interrupting a live turn and resuming settle asynchronously (the
// kill, the wait goroutine, the resume), so a snapshot right after the call may
// still show the old turn running with the correction waiting. The queue knows
// at decision time which branch it took, so it returns that instead.
type SteerOutcome struct {
	// Steering is true when the correction runs first now: it interrupted a live
	// turn, or it resumed a settled worker. Position is 0 in that case.
	Steering bool
	// Position is the correction's 1-based slot in the inbox when the worker had
	// no turn to interrupt yet (queued or blocked), so the correction waits at
	// the front and runs first when the worker's turn settles. It is always 1
	// (the front), reported for parity with send's position.
	Position int
}

// ErrNotSteerable refuses a steer for a worker that has no live turn to correct
// and no session to resume: it was cancelled, it failed, or it settled without a
// captured session id. The caller should start a new worker with delegate (or
// send a follow-up to a still-resumable one).
var ErrNotSteerable = errors.New("that worker cannot be steered: it was cancelled, failed, or captured no session to resume")

// steerInterruptNote is the resolution recorded for the turn a steer stopped, so
// the interrupted turn's delivery says plainly what happened and what was lost:
// any work the turn had not yet written to the session is gone, while anything
// it wrote to the worktree stays. It carries no task text.
const steerInterruptNote = "this turn was interrupted by steer so a correction could run first; work the turn had not yet saved to the session is lost, but anything it wrote to the worktree remains:\n"

// steerDroppedNote is the resolution recorded when a steered turn captured no
// session id, so the correction cannot resume and is dropped with the worker.
// It tells the caller the steer did not take and the worker must be restarted.
const steerDroppedNote = "this turn was interrupted by steer, but it captured no session to resume, so the correction could not be applied and the worker has ended; start a new worker with delegate:\n"

// Steer applies a correction to a worker, running it ahead of any queued turn.
// It validates the message like a turn (non-empty, within the byte cap, valid
// UTF-8), then acts by the worker's state as described on the package doc
// above. It refuses an unknown id and a worker that is neither live nor
// resumable, changing nothing in that case.
func (q *Queue) Steer(id, message string) (SteerOutcome, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return SteerOutcome{}, ErrEmptyTask
	}
	if !utf8.ValidString(message) || len(message) > MaxTaskBytes {
		return SteerOutcome{}, ErrTaskTooLong
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	j, ok := q.jobs[id]
	if !ok {
		return SteerOutcome{}, ErrUnknownJob
	}
	if !q.canSteer(j) {
		return SteerOutcome{}, ErrNotSteerable
	}
	if len(j.inbox) >= MaxInbox {
		return SteerOutcome{}, ErrInboxFull
	}
	if err := q.admitTurnLocked(j); err != nil {
		return SteerOutcome{}, err
	}
	// The correction always goes to the FRONT, so it runs before whatever was
	// queued. On a live turn that means it is the first thing the resumed session
	// sees after the interrupt; on a queued/blocked/settled worker it jumps the
	// queue the same way.
	j.inbox = append([]string{message}, j.inbox...)
	switch {
	case j.state == Running:
		// A live turn: interrupt it. Mark the job steering so wait settles it
		// resumable (not Failed) when the killed process exits, then kill. The
		// front-of-inbox correction resumes the session from settleSteeredLocked.
		j.steering = true
		if j.proc != nil {
			_ = j.proc.Kill()
		}
		q.notify()
		return SteerOutcome{Steering: true}, nil
	case j.state.Finished():
		// Settled and resumable (canSteer checked it): the correction is the only
		// inbox entry, so it is the next turn. It starts now when a slot is free,
		// or waits at the front of the queue within the concurrency limit.
		q.queueResumeLocked(j)
		q.pump()
		q.notify()
		if j.state == Running {
			return SteerOutcome{Steering: true}, nil
		}
		return SteerOutcome{Position: 1}, nil
	}
	// Queued or Blocked: nothing to start now. A queued worker starts its first
	// turn through pump and drains the inbox when it settles; a blocked worker
	// resumes when it is answered. The correction waits at the front (position 1)
	// and runs first either way.
	q.notify()
	return SteerOutcome{Position: 1}, nil
}

// canSteer reports whether a worker can be steered. The caller holds the lock.
// A live worker (queued, running, blocked) always can: the correction waits at
// the front of its inbox. A settled worker can only when it is still resumable,
// which is exactly the continuation rule (canResume): it ended on its own with a
// captured session and the launcher can resume. A cancelled or failed worker, or
// one with no session id, cannot be steered.
func (q *Queue) canSteer(j *job) bool {
	return q.canResume(j)
}

// settleSteeredLocked ends the turn a steer interrupted and hands the session to
// the front-of-inbox correction. The caller holds the lock and has set steering
// and killed the process; this runs from wait once the killed process exits.
//
// The interrupt is not a failure: the turn settled on purpose so a correction
// could run first. So it is settled Done, which lets finishLocked capture the
// session id from the process and drainInboxLocked resume the front-of-inbox
// correction as the next turn. If the turn captured no session id (it was killed
// before its CLI wrote one), there is nothing to resume: the job settles and the
// correction is dropped with it, and the delivery says so. The caller keeps no
// turn text on disk; the worktree writes the turn already made stay (R6).
func (q *Queue) settleSteeredLocked(j *job) {
	j.steering = false
	note := steerInterruptNote
	// Peek at whether the session will be captured: finishLocked reads it from
	// the process, so read the same source here to word the note honestly.
	if sr, ok := j.proc.(sessionReporter); !ok || sr.SessionID() == "" {
		note = steerDroppedNote
	}
	q.finishLocked(j, Done, note+j.proc.Report())
}
