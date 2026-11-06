// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrNoBudget refuses a job without a time limit: a hung worker would hold a
// slot forever (spec R24).
var ErrNoBudget = errors.New("a worker needs a time budget above zero")

// Queue starts jobs up to a limit and keeps their results until acknowledged.
type Queue struct {
	mu       sync.Mutex
	launcher Launcher
	limit    int
	maxJobs  int // total jobs a session may start; 0 = no cap
	now      func() time.Time
	jobs     map[string]*job
	waiting  []string
	running  int
	nextID   int
	deliv    deliveries
	changed  chan struct{}
	// cancelling holds pump back while Cancel settles a batch, so a job that is
	// about to be cancelled is never started on the way.
	cancelling bool
	// spend is the per-agent token ledger and budget (budget.go).
	spend spend
	// turns counts the follow-up turns send and steer accepted, against the
	// session's turn cap (queue_cap.go).
	turns int
	// waves holds the token budget and spend of each delegate call (wave.go).
	waves waves
	// reports is the per-worker report that outlives release (report.go).
	reports reports
}

// New returns a queue that runs at most limit workers at once (at least one).
func New(launcher Launcher, limit int, now func() time.Time) *Queue {
	if limit < 1 {
		limit = 1
	}
	return &Queue{
		launcher: launcher,
		limit:    limit,
		maxJobs:  DefaultMaxJobs,
		now:      now,
		jobs:     map[string]*job{},
		spend:    spend{used: map[string]int64{}},
		waves:    waves{byNum: map[int]*wave{}},
		reports:  reports{byJob: map[string]*Report{}}, deliv: newDeliveries(now()),
		changed: make(chan struct{}),
	}
}

// Delegate queues one job per spec, starts what the limit allows and returns
// the job ids in order. It returns at once; results come through Collect.
func (q *Queue) Delegate(specs []Spec) ([]string, error) {
	ids, _, err := q.DelegateWave(specs, WaveLimits{})
	return ids, err
}

// Cancel stops the named jobs and returns the ids it acted on. A job that has
// already ended is skipped, not reported as an error. A cancelled worker will
// not run another turn, so its on-disk session transcript is deleted here (the
// isolation cleanup): a cancelled worker leaves nothing behind in the user's
// resume list.
func (q *Queue) Cancel(ids []string) []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	var acted []string
	q.cancelling = true
	for _, id := range ids {
		j, ok := q.jobs[id]
		if !ok || j.state.Finished() {
			continue
		}
		hadProc := j.proc != nil
		if hadProc {
			_ = j.proc.Kill()
		}
		q.finishLocked(j, Cancelled, "cancelled by the caller")
		// A worker that never started a process persisted no session, so clean up
		// now with whatever id is on the job (none, for a queued worker). A worker
		// with a process may still write its session id as it dies; the wait
		// goroutine captures that late id and runs the deletion then, so mark it
		// pending here rather than cleaning before the id is known.
		if hadProc {
			j.cleanupPending = true
		} else {
			_ = q.cleanupSessionLocked(j)
		}
		acted = append(acted, id)
	}
	q.cancelling = false
	q.pump()
	q.notify()
	return acted
}

// Statuses returns every known job in start order, without prompts.
func (q *Queue) Statuses() []Status {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Status, 0, len(q.jobs))
	for i := 1; i <= q.nextID; i++ {
		id := jobID(i)
		if j, ok := q.jobs[id]; ok {
			out = append(out, Status{ID: j.id, Agent: j.spec.Agent, State: j.state})
		}
	}
	return out
}

// jobID is the stable id of the nth job (1-based): w001, w002, ...
func jobID(n int) string { return fmt.Sprintf("w%03d", n) }

// active counts jobs that have not ended. The caller holds the lock.
func (q *Queue) active() int {
	n := 0
	for _, j := range q.jobs {
		if !j.state.Finished() {
			n++
		}
	}
	return n
}

// pump starts waiting jobs while there is room. The caller holds the lock.
func (q *Queue) pump() {
	if q.cancelling {
		return
	}
	for q.running < q.limit && len(q.waiting) > 0 {
		id := q.waiting[0]
		q.waiting = q.waiting[1:]
		if j := q.jobs[id]; j.state == Queued && j.resume {
			q.resumeNext(j)
		} else if j.state == Queued {
			q.start(j)
		}
	}
}

// start launches one job and arms its time budget. The caller holds the lock.
func (q *Queue) start(j *job) {
	proc, err := q.launcher.Launch(j.spec)
	if err != nil {
		q.finishLocked(j, Failed, "start failed: "+err.Error())
		return
	}
	j.state, j.proc = Running, proc
	if wr, ok := proc.(worktreeReporter); ok {
		j.worktree, j.base = wr.worktreeInfo()
	}
	// Record the id the launcher knew upfront (session_capture.go) so a first-turn
	// cancel, killed before the CLI writes output, still has an id to clean up by.
	q.recordPresetSessionLocked(j, proc)
	j.started = q.now()
	q.runLocked(j, proc)
}

// finishLocked settles a job once, releases its slot, queues its delivery and
// starts the next waiting job. The caller holds the lock.
func (q *Queue) finishLocked(j *job, s State, text string) {
	if j.state.Finished() {
		return
	}
	if j.state == Running {
		q.running--
	}
	if j.timer != nil {
		j.timer.Stop()
	}
	j.state = s
	j.ended = q.now()
	var tokens *Tokens
	if j.proc != nil {
		tokens = j.proc.Tokens()
		// Capture the opaque session id when the process reports one, so a later
		// turn can resume this conversation. Read through the optional interface,
		// the same way the worktree info is read: a Process that does not report a
		// session is simply not resumable.
		if sr, ok := j.proc.(sessionReporter); ok {
			j.sessionID = sr.SessionID()
		}
	}
	// Keep a trimmed copy for the status view, which outlives the delivery: the
	// full text goes out through collect and is dropped on acknowledgement.
	j.tokens = tokens
	j.summary = trimSummary(text)
	q.spend.record(j.spec.Agent, s, tokens)
	q.waves.spend(j.wave, tokens)
	q.reports.settle(j, len(capText(text)))
	q.deliv.add(j.id, string(s), text, tokens, j.sessionID)
	// A worker that ended on its own with queued turns resumes its next turn
	// here, so the inbox is served one turn at a time in arrival order. A
	// cancelled or failed worker does not resume: drainInboxLocked checks
	// canResume, so its queued turns are dropped with the job (continuation.go).
	// The drain runs before pump so a resumed turn takes its own slot back
	// before another queued job starts.
	q.drainInboxLocked(j)
	q.pump()
	q.notify()
}

// notify wakes every waiter. The caller holds the lock.
func (q *Queue) notify() {
	close(q.changed)
	q.changed = make(chan struct{})
}
