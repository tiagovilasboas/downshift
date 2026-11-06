// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"fmt"
	"time"
)

// One turn's run: a started turn is counted, its time budget armed and its exit
// awaited. Every step carries the turn number, so a stale timer or a late exit
// of an earlier turn never settles a later one (a resumed follow-up).

// runLocked counts a started turn, arms its time budget and waits for its exit.
// Both carry the turn number, so a late exit or timer of an earlier turn never
// settles this one. The caller holds the lock and has set the job Running.
func (q *Queue) runLocked(j *job, proc Process) {
	q.running++
	j.turn++
	turn := j.turn
	j.timer = time.AfterFunc(j.spec.Budget, func() { q.expire(j.id, turn) })
	go q.wait(j.id, proc, turn)
}

// wait blocks until a worker exits and records how it ended. A job that was
// cancelled or timed out has already been settled, so the late exit is ignored.
// A steered worker is a special case: its live turn was interrupted on purpose
// (steer.go), so even though the process exited with a kill error the turn is
// settled resumable, not Failed, and the front-of-inbox correction resumes the
// session. See settleSteeredLocked for the detail.
func (q *Queue) wait(id string, proc Process, turn int) {
	err := proc.Wait()
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.jobs[id]
	if j == nil || j.turn != turn {
		// The job settled (cancelled or timed out) and was released before its
		// process exited: release already dropped it, so the late exit has
		// nothing left to record. Or a later turn already started (a timed-out
		// turn whose queued follow-up resumed): this exit is not that turn's.
		return
	}
	if j.state.Finished() {
		// The job was already settled (cancelled or timed out) before its process
		// exited. The process has now flushed its output, so capture any late
		// session id it wrote: a timed-out worker's transcript is deleted on
		// release, which needs that id, and a cancelled worker's is deleted now.
		if sr, ok := proc.(sessionReporter); ok && j.sessionID == "" {
			j.sessionID = sr.SessionID()
		}
		if j.cleanupPending {
			j.cleanupPending = false
			_ = q.cleanupSessionLocked(j)
		}
		return
	}
	if j.steering {
		q.settleSteeredLocked(j)
		return
	}
	if err != nil {
		q.finishLocked(j, Failed, failureText(err, proc))
		return
	}
	q.finishLocked(j, Done, proc.Report())
}

// expire ends a job whose budget ran out, with what it had written so far.
func (q *Queue) expire(id string, turn int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.jobs[id]
	if j == nil || j.state != Running || j.turn != turn {
		return
	}
	reason := fmt.Sprintf("time budget of %s ran out; partial output follows:\n", j.spec.Budget)
	_ = j.proc.Kill()
	q.finishLocked(j, TimedOut, reason+j.proc.Report())
}

// failureText is the reason a worker failed: what it said, then its stderr tail.
func failureText(err error, proc Process) string {
	text := err.Error()
	if tail := proc.Stderr(); tail != "" {
		text += "\nstderr: " + tail
	}
	return text
}
