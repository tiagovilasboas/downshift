// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sort"
	"time"
)

// Task groups. A spawn the built-in control group left unrouted is "control";
// every other spawn that routing was allowed to decide on is "routed" (even
// when the decision was to leave it alone, so the comparison is by intent).
// A baseline spawn with no CONTROL code (DOWNSHIFT_NO_ROUTE, a session pause)
// is "other": it is neither random nor routed and stays out of the comparison.
const (
	GroupRouted  = "routed"
	GroupControl = "control"
	GroupOther   = "other"
)

// TaskRecord joins, for one spawn, the routing decision, the priced usage and
// how the child ended. Ids and numbers only: no prompt, task text or path.
type TaskRecord struct {
	Key     string
	Group   string
	Harness string
	Session string
	Time    time.Time
	// Status is how the child ended, or "" when no end was observed.
	Status ChildStatus
	// CostUSD is the catalog-priced cost of this spawn alone; Priced is false
	// when no usage record carried a cost.
	CostUSD float64
	Priced  bool
	// Redo lists the failed spawns this one redid (a respawn in the same
	// session within the respawn window after an error or interrupt).
	Redo []string
	// Reworked is set on a failed spawn that a later spawn redid.
	Reworked bool
	// TotalUSD is CostUSD plus the cost of every failed attempt it redid,
	// transitively. For a completed task this is its cost including rework.
	TotalUSD float64
}

// Failed reports an error or interrupt end.
func (t TaskRecord) Failed() bool { return failedEnd(t.Status) }

// Completed reports a child that finished normally.
func (t TaskRecord) Completed() bool { return t.Status == ChildCompleted }

func isTaskDecision(ev Event) bool {
	if ev.Harness == "" {
		return false
	}
	// A baseline record is a decision here: the control group is a spawn that
	// was classified and left unchanged.
	switch ev.Outcome {
	case OutcomeUsage, OutcomeResolved, OutcomeChildEnded, "error", OutcomeFailOpen, OutcomeBlocked:
		return false
	}
	return true
}

func hasReason(codes []string, want string) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func groupOf(ev Event) string {
	if ev.Outcome != OutcomeBaseline {
		return GroupRouted
	}
	if hasReason(ev.ReasonCodes, ReasonControl) {
		return GroupControl
	}
	return GroupOther
}

// endRank orders end statuses so the strongest observation wins when a child
// has more than one child_ended record.
func endRank(s ChildStatus) int {
	switch {
	case failedEnd(s):
		return 4
	case s == ChildCompleted:
		return 3
	case s == ChildStopped:
		return 2
	case s == ChildLaunched:
		return 1
	}
	return 0
}

// BuildTasks joins decisions, usage and child ends into one record per spawn,
// in event-time order. Rework is linked with RespawnAfterFailure.
func BuildTasks(events []Event) []TaskRecord {
	var tasks []TaskRecord
	index := map[string]int{}
	for _, ev := range events {
		if !isTaskDecision(ev) {
			continue
		}
		t, ok := parseEventTimeAny(ev.Timestamp)
		if !ok {
			continue
		}
		key := DecisionKey(ev)
		if _, dup := index[key]; dup {
			continue
		}
		index[key] = len(tasks)
		tasks = append(tasks, TaskRecord{
			Key: key, Group: groupOf(ev), Harness: ev.Harness, Session: ev.SessionID, Time: t,
		})
	}
	for _, ev := range events {
		if ev.LinkedDecision == "" {
			continue
		}
		i, ok := index[ev.LinkedDecision]
		if !ok {
			continue
		}
		switch ev.Outcome {
		case OutcomeUsage:
			if ev.ActualCostUSD != nil {
				tasks[i].CostUSD += *ev.ActualCostUSD
				tasks[i].Priced = true
			}
			// Background children report only "launched" at spawn time; the
			// usage record written when the child stops is the proof that it ran
			// to the end. It counts as completed unless a failure end outranks it.
			if endRank(ChildCompleted) > endRank(tasks[i].Status) {
				tasks[i].Status = ChildCompleted
			}
		case OutcomeChildEnded:
			if endRank(ev.ChildStatus) > endRank(tasks[i].Status) {
				tasks[i].Status = ev.ChildStatus
			}
		}
	}
	for _, r := range RespawnAfterFailure(events, respawnWindow) {
		f, okF := index[r.FailedKey]
		n, okN := index[r.NextKey]
		if !okF || !okN || f == n || !tasks[n].Time.After(tasks[f].Time) {
			continue
		}
		tasks[f].Reworked = true
		tasks[n].Redo = append(tasks[n].Redo, tasks[f].Key)
	}
	memo := map[int]float64{}
	var total func(i int) float64
	total = func(i int) float64 {
		if v, ok := memo[i]; ok {
			return v
		}
		sum := tasks[i].CostUSD
		for _, k := range tasks[i].Redo {
			// A redo is always later than the attempt it redid, so this
			// recursion strictly moves back in time and cannot cycle.
			sum += total(index[k])
		}
		memo[i] = sum
		return sum
	}
	for i := range tasks {
		tasks[i].TotalUSD = total(i)
	}
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].Time.Before(tasks[j].Time) })
	return tasks
}

func parseEventTimeAny(s string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
	}
	return t, err == nil
}
