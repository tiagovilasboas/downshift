// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"strings"
	"time"
	"unicode/utf8"
)

// maxSummary bounds the result or error text the status view carries per
// worker. It is far smaller than a delivery's 16 000-byte cap: the status view
// is a glance at what runs, not a place to read a worker's full report (spec
// R21 trims every status field). Read the full text with collect.
const maxSummary = 240

// WorkerView is one worker as the status tool shows it: enough to see what runs
// without calling collect and waiting. It carries no prompt and no model name
// the job did not record. Elapsed ticks while the worker runs and freezes when
// it ends.
type WorkerView struct {
	// ID is the job id (w001, w002, ...).
	ID string
	// State is queued, running, done, failed, cancelled or timeout.
	State State
	// Agent is the worker CLI (claude, codex); Harness is its strain block id.
	Agent   string
	Harness string
	// Write is true when the worker runs in its own git worktree (it can write);
	// false when it is read-only.
	Write bool
	// Elapsed is how long the worker has run: now - start while running, or
	// end - start once finished. Zero before it starts (still queued).
	Elapsed time.Duration
	// Tokens is the usage the worker reported once it ended; nil while running
	// or when its CLI wrote no usage.
	Tokens *Tokens
	// Summary is a short, trimmed copy of the result or error once the worker
	// ends; empty while it runs. The full text comes through collect.
	Summary string
}

// Workers returns a view of every known job in start order, holding the lock
// only long enough to copy the fields, so a status call never serializes
// behind a waiting collect (spec R22). It is read-only: it changes no state and
// never touches a running process.
func (q *Queue) Workers() []WorkerView {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]WorkerView, 0, len(q.jobs))
	for i := 1; i <= q.nextID; i++ {
		id := jobID(i)
		j, ok := q.jobs[id]
		if !ok {
			continue
		}
		out = append(out, q.viewOf(j))
	}
	return out
}

// viewOf copies one job into a view. The caller holds the lock.
func (q *Queue) viewOf(j *job) WorkerView {
	return WorkerView{
		ID:      j.id,
		State:   j.state,
		Agent:   j.spec.Agent,
		Harness: harnessFor(j.spec.Agent),
		Write:   j.spec.Write,
		Elapsed: q.elapsed(j),
		Tokens:  j.tokens,
		Summary: j.summary,
	}
}

// elapsed is how long a job has run. A queued job has not started, so it is
// zero; a running job is measured to now; a finished job is frozen at its end.
// The caller holds the lock.
func (q *Queue) elapsed(j *job) time.Duration {
	if j.started.IsZero() {
		return 0
	}
	end := j.ended
	if end.IsZero() {
		end = q.now()
	}
	d := end.Sub(j.started)
	if d < 0 {
		return 0
	}
	return d
}

// harnessFor maps a worker agent to the harness whose strain block describes
// it, so the status view names the same harness the delegate result does. An
// agent without a mapping returns itself.
func harnessFor(agent string) string {
	switch agent {
	case "claude":
		return "claude-code"
	case "codex":
		return "codex"
	default:
		return agent
	}
}

// trimSummary keeps the first maxSummary bytes of a worker's text, cut on a
// character boundary and collapsed onto one glance: leading and trailing space
// gone and inner runs of blank space squeezed to a single space, so a status
// view stays compact. It says when it cut.
func trimSummary(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	if len(s) <= maxSummary {
		return s
	}
	cut := maxSummary
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}
