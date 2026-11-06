// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"sort"
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
	// SessionID is the opaque id the worker's CLI wrote, so a later turn can
	// resume the same conversation; empty while it runs and empty when the CLI
	// wrote none. The status view shows it so a caller can see which workers are
	// resumable without calling collect.
	SessionID string
	// Pending is how many more turns are queued for this worker (continuation.go):
	// turns a caller enqueued while a turn was running, waiting to resume one at
	// a time. Zero when the inbox is empty. It lets a caller see pending work
	// without calling collect.
	Pending int
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
		ID:        j.id,
		State:     j.state,
		Agent:     j.spec.Agent,
		Harness:   HarnessFor(j.spec.Agent),
		Write:     j.spec.Write,
		Elapsed:   q.elapsed(j),
		Tokens:    j.tokens,
		Summary:   j.summary,
		SessionID: j.sessionID,
		Pending:   len(j.inbox),
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

// agentHarness maps each worker agent to the harness whose strain block
// describes it. The harness table (core.HarnessCapabilities.WorkerAgent) owns
// the pairing; this package does not import core, so the binary installs the
// map with SetAgentHarnesses at start-up. The status view, the delegate check
// and every work tool's strain block read it from here.
var agentHarness = map[string]string{}

// SetAgentHarnesses installs the agent to harness map, replacing any earlier
// one. Call it once at start-up, before the queue runs.
func SetAgentHarnesses(m map[string]string) {
	agentHarness = make(map[string]string, len(m))
	for a, h := range m {
		agentHarness[a] = h
	}
}

// HarnessFor returns the harness whose strain block describes a worker agent.
// An agent without a mapping returns itself.
func HarnessFor(agent string) string {
	if h, ok := agentHarness[agent]; ok {
		return h
	}
	return agent
}

// KnownAgent reports whether agent is a worker agent the queue can run.
func KnownAgent(agent string) bool {
	_, ok := agentHarness[agent]
	return ok
}

// WorkerHarnesses lists the harness of every worker agent, sorted, for a strain
// block that covers all of them.
func WorkerHarnesses() []string {
	out := make([]string, 0, len(agentHarness))
	for _, h := range agentHarness {
		out = append(out, h)
	}
	sort.Strings(out)
	return out
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
