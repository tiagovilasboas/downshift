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
}

// New returns a queue that runs at most limit workers at once (at least one).
func New(launcher Launcher, limit int, now func() time.Time) *Queue {
	if limit < 1 {
		limit = 1
	}
	return &Queue{
		launcher: launcher,
		limit:    limit,
		now:      now,
		jobs:     map[string]*job{},
		spend:    spend{used: map[string]int64{}},
		deliv:    newDeliveries(now()),
		changed:  make(chan struct{}),
	}
}

// Delegate queues one job per spec, starts what the limit allows and returns
// the job ids in order. It returns at once; results come through Collect.
func (q *Queue) Delegate(specs []Spec) ([]string, error) {
	for _, s := range specs {
		if s.Budget <= 0 {
			return nil, ErrNoBudget
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, s := range specs {
		if err := q.spend.check(s.Agent); err != nil {
			return nil, err
		}
	}
	ids := make([]string, 0, len(specs))
	for _, s := range specs {
		q.nextID++
		id := fmt.Sprintf("w%03d", q.nextID)
		s.ID = id
		q.jobs[id] = &job{id: id, spec: s, state: Queued}
		q.waiting = append(q.waiting, id)
		ids = append(ids, id)
	}
	q.pump()
	q.notify()
	return ids, nil
}

// Cancel stops the named jobs and returns the ids it acted on. A job that has
// already ended is skipped, not reported as an error.
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
		if j.proc != nil {
			_ = j.proc.Kill()
		}
		q.finishLocked(j, Cancelled, "cancelled by the caller")
		acted = append(acted, id)
	}
	q.cancelling = false
	q.pump()
	q.notify()
	return acted
}

// Close stops every job that has not ended: a running worker is killed and a
// queued one never starts. The server calls it when it exits, so no worker
// outlives the session that started it.
func (q *Queue) Close() {
	q.mu.Lock()
	ids := make([]string, 0, len(q.jobs))
	for id, j := range q.jobs {
		if !j.state.Finished() {
			ids = append(ids, id)
		}
	}
	q.mu.Unlock()
	q.Cancel(ids)
}

// Statuses returns every known job in start order, without prompts.
func (q *Queue) Statuses() []Status {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]Status, 0, len(q.jobs))
	for i := 1; i <= q.nextID; i++ {
		id := fmt.Sprintf("w%03d", i)
		if j, ok := q.jobs[id]; ok {
			out = append(out, Status{ID: j.id, Agent: j.spec.Agent, State: j.state})
		}
	}
	return out
}

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
		if j := q.jobs[id]; j.state == Queued {
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
	q.running++
	j.timer = time.AfterFunc(j.spec.Budget, func() { q.expire(j.id) })
	go q.wait(j.id, proc)
}

// wait blocks until a worker exits and records how it ended. A job that was
// cancelled or timed out has already been settled, so the late exit is ignored.
func (q *Queue) wait(id string, proc Process) {
	err := proc.Wait()
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.jobs[id]
	if j.state.Finished() {
		return
	}
	if err != nil {
		q.finishLocked(j, Failed, failureText(err, proc))
		return
	}
	q.finishLocked(j, Done, proc.Report())
}

// expire ends a job whose budget ran out, with what it had written so far.
func (q *Queue) expire(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	j := q.jobs[id]
	if j == nil || j.state != Running {
		return
	}
	reason := fmt.Sprintf("time budget of %s ran out; partial output follows:\n", j.spec.Budget)
	_ = j.proc.Kill()
	q.finishLocked(j, TimedOut, reason+j.proc.Report())
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
	var tokens *Tokens
	if j.proc != nil {
		tokens = j.proc.Tokens()
	}
	q.spend.record(j.spec.Agent, s, tokens)
	q.deliv.add(j.id, string(s), text, tokens)
	q.pump()
	q.notify()
}

// notify wakes every waiter. The caller holds the lock.
func (q *Queue) notify() {
	close(q.changed)
	q.changed = make(chan struct{})
}

// failureText is the reason a worker failed: what it said, then its stderr tail.
func failureText(err error, proc Process) string {
	text := err.Error()
	if tail := proc.Stderr(); tail != "" {
		text += "\nstderr: " + tail
	}
	return text
}
