// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Continuation is the state that lets a worker run another turn on the same
// conversation, without re-paying to re-read its context. It is in-queue state,
// not a new worker: a settled worker that captured a session id is resumable,
// and a live worker collects the next turn in its inbox and runs it when the
// current turn ends. The queue holds only opaque values (a session id, a task
// string) and asks the launcher to resume; it never learns whether the worker
// is Claude or Codex, and a resumed turn reuses the original spec exactly, so
// it never changes dir, model, effort or mode and never climbs a tier.
//
// This is the base other continuation tools build on (send, steer, release,
// tasks T3-T5 of docs/internal/plans/mcp-worker-continuation.md). Continue is
// the internal "queue more work for job X"; no MCP tool is wired here.

// MaxInbox bounds how many turns can wait for one worker, so a caller cannot
// grow the queue's memory without bound by enqueuing forever. It matches the
// delegate call's per-call task cap (maxTasksPerCall): a worker holds at most
// as many queued turns as a single delegate starts workers.
const MaxInbox = 8

// MaxTaskBytes bounds one turn's text, the same cap delegate puts on a task
// string (the delegateSchema maxLength). A turn over it is refused before it
// reaches a worker.
const MaxTaskBytes = 4000

var (
	// ErrUnknownJob refuses a continuation for an id the queue never issued.
	ErrUnknownJob = errors.New("no worker has that id")
	// ErrNotResumable refuses a continuation for a worker that cannot take
	// another turn: it was cancelled, it failed, or it settled without a
	// captured session id (so there is no conversation to resume).
	ErrNotResumable = errors.New("that worker cannot take another turn: it was cancelled, failed, or captured no session to resume")
	// ErrEmptyTask refuses a turn with no instruction.
	ErrEmptyTask = errors.New("a turn needs a task")
	// ErrTaskTooLong refuses a turn whose text is over the per-turn byte cap.
	ErrTaskTooLong = errors.New("the turn text is over the size limit")
	// ErrInboxFull refuses a turn when the worker already has the most it may
	// hold waiting.
	ErrInboxFull = errors.New("the worker already has the most queued turns it may hold")
)

// resumeLauncher is the optional contract a Launcher satisfies when it can
// continue a stored session with a new turn. The queue passes the original spec
// (with only Task replaced by the new turn) and the opaque session id; the
// launcher owns how to turn that into a resumed process for its CLI (the
// ResumeArgs it built next to the launch). A Launcher that does not implement
// it cannot resume, and Continue refuses a settled worker under it. Keeping the
// resume behind the launcher is the same owner split as the launch: the queue
// stays agnostic.
type resumeLauncher interface {
	LaunchResume(spec Spec, sessionID string) (Process, error)
}

// Continue queues one more turn of work for an existing worker, on the same
// conversation. It is the internal base the send tool will call (T3); no MCP
// tool is wired here.
//
//   - Worker still running (or blocked): the turn joins the worker's inbox and
//     runs when the live turn settles. This never interrupts the running turn
//     and never blocks collect, cancel or status (they take the same lock only
//     to read or settle, not for the turn's lifetime).
//   - Worker already settled and resumable: the turn resumes its session now,
//     reusing the original spec, and its result comes back through collect as a
//     new delivery on the same job id.
//
// It refuses an unknown id, a worker that cannot resume (cancelled, failed, or
// no captured session id), an empty or oversized turn, and a full inbox, each
// with a clear error and no change to the queue.
func (q *Queue) Continue(id, task string) error {
	task = strings.TrimSpace(task)
	if task == "" {
		return ErrEmptyTask
	}
	if !utf8.ValidString(task) || len(task) > MaxTaskBytes {
		return ErrTaskTooLong
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	j, ok := q.jobs[id]
	if !ok {
		return ErrUnknownJob
	}
	if !q.canResume(j) {
		return ErrNotResumable
	}
	if len(j.inbox) >= MaxInbox {
		return ErrInboxFull
	}
	if err := q.admitTurnLocked(j); err != nil {
		return err
	}
	j.inbox = append(j.inbox, task)
	// A live worker runs its inbox when its turn settles; a settled worker has
	// no turn to wait on, so its next turn joins the queue now and starts when
	// a slot is free.
	if j.state.Finished() {
		q.queueResumeLocked(j)
		q.pump()
	}
	q.notify()
	return nil
}

// admitTurnLocked applies the same limits to a follow-up turn (send or steer)
// that delegate applies to a new worker: the agent's token budget, the wave's
// token budget (wave.go) and the
// session's turn cap. The concurrency limit applies when the turn starts, since
// a resumed turn waits for a slot like any queued job. On success it counts
// the turn. The caller holds the lock.
func (q *Queue) admitTurnLocked(j *job) error {
	if err := q.spend.check(j.spec.Agent); err != nil {
		return err
	}
	if err := q.waves.check(j.wave); err != nil {
		return err
	}
	if c := q.turnCap(); c > 0 && q.turns >= c {
		return ErrTurnCapReached
	}
	q.turns++
	return nil
}

// queueResumeLocked puts a settled worker with queued turns back in the queue,
// at the front, so its next turn starts through pump within the concurrency
// limit: a follow-up never runs past the limit, and it starts before a new job
// since the worker held a slot a moment ago. The finished process is dropped:
// it has exited and its tokens and session id were read at settle, so a cancel
// of the waiting turn cleans the session up at once. The caller holds the lock.
func (q *Queue) queueResumeLocked(j *job) {
	j.state, j.resume, j.proc = Queued, true, nil
	q.waiting = append([]string{j.id}, q.waiting...)
}

// canResume reports whether a worker can take another turn. The caller holds
// the lock. A live worker (queued, running, blocked) always can: the turn waits
// in the inbox. A settled worker can only when it ended on its own (done or
// timeout, not cancelled or failed) and captured a session id, and the launcher
// can resume at all. A cancelled or failed worker, or one with no session id,
// cannot: there is no conversation to continue.
func (q *Queue) canResume(j *job) bool {
	if !j.state.Finished() {
		return true
	}
	if j.sessionID == "" {
		return false
	}
	if j.state != Done && j.state != TimedOut {
		return false
	}
	_, ok := q.launcher.(resumeLauncher)
	return ok
}

// resumeNext starts the worker's next inbox turn as a resume of its session,
// reusing the original spec with only the task replaced, so the turn runs in
// the same dir with the same model, effort and mode: a resumed turn never
// climbs a tier or widens a mode. pump calls it for a queued resume when a slot
// is free; the caller holds the lock. The budget is checked again here, since
// the turn before may have spent it: a spent budget, a launcher that cannot
// resume or an empty inbox settles the job Failed with the reason.
func (q *Queue) resumeNext(j *job) {
	j.resume = false
	rl, ok := q.launcher.(resumeLauncher)
	if !ok || len(j.inbox) == 0 {
		q.finishLocked(j, Failed, "resume failed: no turn to resume")
		return
	}
	if err := q.spend.check(j.spec.Agent); err != nil {
		q.finishLocked(j, Failed, "resume refused: "+err.Error())
		return
	}
	task := j.inbox[0]
	j.inbox = j.inbox[1:]
	spec := j.spec
	spec.Task = task
	proc, err := rl.LaunchResume(spec, j.sessionID)
	if err != nil {
		q.finishLocked(j, Failed, "resume failed: "+err.Error())
		return
	}
	j.state, j.proc = Running, proc
	// A resumed turn keeps the same session id; record the launcher's upfront id
	// so a cancel mid-resume still cleans up by it, the same as a first turn.
	q.recordPresetSessionLocked(j, proc)
	j.started = q.now()
	j.ended = time.Time{} // the previous turn's end no longer applies
	q.runLocked(j, proc)
}

// drainInboxLocked queues the worker's next turn, if any, after its current
// turn has settled. It is called from finishLocked once the job is settled: a
// settled job with a non-empty inbox and a captured session goes back to the
// front of the queue in place of staying terminal, and the pump that follows
// starts it in the slot the turn just freed, so the inbox is served one turn at
// a time in arrival order. A cancelled or failed settlement does not drain:
// canResume is false, so the queued turns are dropped with the job. The caller
// holds the lock.
func (q *Queue) drainInboxLocked(j *job) {
	if len(j.inbox) == 0 || !q.canResume(j) {
		return
	}
	q.queueResumeLocked(j)
}
