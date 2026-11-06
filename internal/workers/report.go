// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import "sort"

// A report is the small record the queue keeps for each worker once it ends:
// how much result it left, how many tokens it spent, whether its transcript is
// still on disk, and whether the lead used or discarded the result. It lives
// in memory for the session and outlives release, so the lead can read what
// every worker cost and gave back. It holds no task text, no result text and no
// path: only sizes, counts and flags.

// Mark is the lead's verdict on a worker's result.
type Mark string

const (
	// MarkNone: the lead has not said.
	MarkNone Mark = ""
	// MarkUsed: the lead used the result.
	MarkUsed Mark = "used"
	// MarkDiscarded: the lead threw the result away.
	MarkDiscarded Mark = "discarded"
)

// ParseMark accepts "", "used" and "discarded".
func ParseMark(s string) (Mark, bool) {
	switch m := Mark(s); m {
	case MarkNone, MarkUsed, MarkDiscarded:
		return m, true
	}
	return MarkNone, false
}

// StatusUnmarked is the report status before the lead marks the worker.
const StatusUnmarked = "unmarked"

// Report is one worker as the status tool lists it.
type Report struct {
	Job   string `json:"job"`
	Agent string `json:"agent"`
	Wave  int    `json:"wave"`
	State State  `json:"state"`
	// ResultBytes is the size of the result the last turn left to collect.
	ResultBytes int `json:"result_bytes"`
	// Tokens is what the worker's turns reported spending, summed; 0 when its
	// CLI wrote no usage.
	Tokens int64 `json:"tokens,omitempty"`
	// TranscriptOnDisk is true while the CLI's session transcript stays on the
	// user's disk: from the end of a resumable turn until release deletes it.
	// The path is never reported.
	TranscriptOnDisk bool `json:"transcript_on_disk"`
	// Status is unmarked, used or discarded.
	Status string `json:"status"`
}

// reports keeps one Report per worker. Touched only under the queue's lock.
type reports struct {
	byJob map[string]*Report
}

// settle records a turn that just ended: the result size, the tokens it
// reported and whether its transcript is on disk.
func (r *reports) settle(j *job, resultBytes int) {
	rep := r.byJob[j.id]
	if rep == nil {
		rep = &Report{Job: j.id, Agent: j.spec.Agent, Wave: j.wave, Status: StatusUnmarked}
		r.byJob[j.id] = rep
	}
	rep.State = j.state
	rep.ResultBytes = resultBytes
	if j.tokens != nil {
		rep.Tokens += j.tokens.Total
	}
	rep.TranscriptOnDisk = j.sessionID != "" && j.state != Cancelled
}

// release records the lead's mark and whether the transcript is gone.
func (r *reports) release(id string, mark Mark, transcriptLeft bool) {
	rep := r.byJob[id]
	if rep == nil {
		return
	}
	rep.TranscriptOnDisk = transcriptLeft
	if mark != MarkNone {
		rep.Status = string(mark)
	}
}

// Reports lists the report of every worker that has ended, in start order,
// including workers already released.
func (q *Queue) Reports() []Report {
	q.mu.Lock()
	defer q.mu.Unlock()
	ids := make([]string, 0, len(q.reports.byJob))
	for id := range q.reports.byJob {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]Report, 0, len(ids))
	for _, id := range ids {
		out = append(out, *q.reports.byJob[id])
	}
	return out
}
