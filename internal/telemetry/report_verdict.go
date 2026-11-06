// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"sort"
)

// VerdictNotRecorded labels routed spawns whose decision holds no routing
// note (written before the note, or by a path that does not record it).
const VerdictNotRecorded = "not_recorded"

// verdictHeldAfterFailure is the label of a spawn the class would have moved
// down but did not, because a spawn Downshift moved down in the same session
// had just failed. The router records it as a kept verdict with this reason;
// the report shows it as its own row because the rework it follows is the one
// the hold exists to prevent.
const verdictHeldAfterFailure = "held_after_failure"

// verdictSwitch names, per routing verdict, the policy key that turns it off.
// An absent verdict has no key: the owner can only read the line.
var verdictSwitch = map[string]string{
	"class":         "routing.by_class: false",
	"class_up":      "routing.raise_complex: false",
	"agent_default": "routing.agent_default_tier: false",
	"rule":          "routing.rules[].enabled: false",
	"profile":       "routing.profiles (remove the profile)",
	"guard_floor":   "min_tier, min_tier_by_class or min_tier_by_risk (remove the floor)",
}

// VerdictRework is the rework of the routed spawns one routing verdict
// decided. Counts and ids only.
type VerdictRework struct {
	Verdict string `json:"verdict"`
	Spawns  int    `json:"spawns"`
	// Ended, Completed, Reworked and ReworkRate read as in GroupSummary.
	Ended      int     `json:"ended"`
	Completed  int     `json:"completed_tasks"`
	Reworked   int     `json:"reworked"`
	ReworkRate float64 `json:"rework_rate"`
	// TurnOff names the policy key that turns the verdict off; absent when no
	// key does. The report never changes the policy.
	TurnOff string `json:"turn_off,omitempty"`
	// ReworkedCorrelationIDs lists the decisions of this verdict that a later
	// spawn redid, in time order, so a misrouted spawn can be found in the log.
	ReworkedCorrelationIDs []string `json:"reworked_correlation_ids,omitempty"`
}

// verdictOf returns the report label of a decision.
func verdictOf(ev Event) string {
	if ev.RoutingNote == nil {
		return VerdictNotRecorded
	}
	n := ev.RoutingNote.Clean()
	switch {
	case n.Verdict == "":
		return VerdictNotRecorded
	case n.Reason == verdictHeldAfterFailure:
		return verdictHeldAfterFailure
	}
	return n.Verdict
}

// verdictTally folds the routed tasks of one window per verdict.
type verdictTally map[string]*VerdictRework

func (v verdictTally) add(t TaskRecord) {
	row := v[t.Verdict]
	if row == nil {
		row = &VerdictRework{Verdict: t.Verdict}
		v[t.Verdict] = row
	}
	row.Spawns++
	if t.Completed() || t.Failed() {
		row.Ended++
	}
	if t.Completed() && t.Observed {
		row.Completed++
	}
	if t.Reworked {
		row.Reworked++
		if t.CorrelationID != "" {
			row.ReworkedCorrelationIDs = append(row.ReworkedCorrelationIDs, t.CorrelationID)
		}
	}
}

// rows closes the tally: most spawns first, then by name, so the same log
// always prints the same order. Each row names the policy key that turns its
// verdict off, when one exists.
func (v verdictTally) rows() []VerdictRework {
	out := make([]VerdictRework, 0, len(v))
	for _, row := range v {
		if row.Ended > 0 {
			row.ReworkRate = float64(row.Reworked) / float64(row.Ended)
		}
		row.TurnOff = verdictSwitch[row.Verdict]
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Spawns != out[j].Spawns {
			return out[i].Spawns > out[j].Spawns
		}
		return out[i].Verdict < out[j].Verdict
	})
	return out
}

// printVerdictRework prints the rework per routing verdict and, on each row,
// the policy key that turns the verdict off. It prints nothing when the window
// holds no routed spawn. Downshift only names the key: the policy stays the
// owner's to change.
func printVerdictRework(w io.Writer, r Report) {
	if len(r.ByVerdict) == 0 {
		return
	}
	fmt.Fprintln(w, "Rework by routing verdict, routed spawns")
	fmt.Fprintf(w, "%-24s %7s %7s %10s %9s  %s\n", "verdict", "spawns", "ended", "completed", "rework", "policy key that turns it off")
	for _, v := range r.ByVerdict {
		fmt.Fprintf(w, "%-24s %7d %7d %10d %9s  %s\n", v.Verdict, v.Spawns, v.Ended, v.Completed,
			reworkCell(v.Ended, v.ReworkRate), turnOffCell(v))
	}
	fmt.Fprintln(w, "  Downshift does not change the policy: the key is in policy.json.")
	fmt.Fprintln(w)
}

func reworkCell(ended int, rate float64) string {
	if ended == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", rate*100)
}

func turnOffCell(v VerdictRework) string {
	if v.TurnOff == "" {
		return "No policy key turns this verdict off"
	}
	return v.TurnOff
}
