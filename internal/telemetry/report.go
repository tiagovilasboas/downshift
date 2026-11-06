// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
)

// GroupSummary is the routed spawns of the window.
type GroupSummary struct {
	Spawns int `json:"spawns"` // decisions in the group
	Ended  int `json:"ended"`  // spawns with an observed end: completed, error or interrupted
	// Completed counts completed tasks with usage observed; only these enter
	// the token figures below.
	Completed int `json:"completed_tasks"`
	// Tokens is the token count of the same tasks (observed usage records, the
	// failed attempts they redid included), by billing class; Tokens.Records is
	// the number of usage records.
	Tokens TokenClasses `json:"tokens"`
	// BillablePerTask is the billable tokens of those tasks (input, output and
	// cache write; TokenClasses.Billable) over Completed. TokensPerTask is
	// Tokens.Total over Completed, cache reads included, kept for reference.
	BillablePerTask float64 `json:"billable_tokens_per_completed_task"`
	TokensPerTask   float64 `json:"tokens_per_completed_task"`
	// Reworked counts failed spawns that were redone; ReworkRate is Reworked
	// over Ended.
	Reworked   int     `json:"reworked"`
	ReworkRate float64 `json:"rework_rate"`
}

// Report is the routed spawns of one window. Prompt-free: counts, rates and
// tokens only. Every spawn is routed, so there is nothing to compare it with:
// the report states what the routed spawns did and where they were redone.
type Report struct {
	Days   int          `json:"days"`
	Routed GroupSummary `json:"routed"`
	// Excluded counts spawns left unrouted by a session pause, and baseline
	// lines older binaries wrote. They are in none of the figures.
	Excluded int `json:"excluded_unrouted"`
	// ByVerdict is the routed group's rework per routing verdict, with the
	// decisions that were followed by rework and the policy key that turns
	// the verdict off.
	ByVerdict []VerdictRework `json:"rework_by_verdict,omitempty"`
	// Completion says, per harness, whether the end of a child is measurable.
	Completion []HarnessCompletion `json:"completion"`
	// Routing is the plan lens: per harness, the billable tokens that ran
	// below the session model in the window, cache read apart, no price at
	// all and no cross-harness total (RoutingTokens).
	Routing RoutingView `json:"routing_tokens"`
	// CompactorSavedBytes and CompactorTokensEstimate are the Compactor's
	// running totals over the whole exec log (not windowed; bytes / 4 is an
	// estimate). TokensSavedEstimate is the final figure: the Compactor's
	// tokens, the only ones that were not spent. See SetCompactor.
	CompactorSavedBytes     int64 `json:"compactor_saved_bytes"`
	CompactorTokensEstimate int64 `json:"compactor_tokens_estimate"`
	TokensSavedEstimate     int64 `json:"tokens_saved_estimate"`
	// DecisionsByPermissionMode counts decisions per permission mode name in
	// the window (see PermissionModeLabel). Display only: no comparison uses it.
	DecisionsByPermissionMode map[string]int `json:"decisions_by_permission_mode,omitempty"`
}

// SetCompactor records the Compactor's totals and recomputes the final
// TokensSavedEstimate. The caller owns the exec log and passes bytes and the
// estimated tokens (bytes / execwrap.BytesPerToken).
func (r *Report) SetCompactor(savedBytes, tokens int64) {
	r.CompactorSavedBytes, r.CompactorTokensEstimate = savedBytes, tokens
	r.TokensSavedEstimate = TokensSavedEstimate(tokens)
}

// BuildReport filters events to the last days (<= 0 means all time) and folds
// the routed spawns. Unrouted spawns are only counted in Excluded.
func BuildReport(events []Event, days int) Report {
	rep := Report{Days: days}
	windowed := FilterByDays(events, days)
	rep.Completion = CompletionByHarness(windowed)
	rep.Routing = RoutingTokens(windowed)
	rep.SetCompactor(0, 0)
	rep.DecisionsByPermissionMode = CountPermissionModes(windowed)
	verdicts := verdictTally{}
	g := &rep.Routed
	for _, t := range BuildTasks(windowed) {
		if t.Group != GroupRouted {
			rep.Excluded++
			continue
		}
		g.Spawns++
		verdicts.add(t)
		if t.Completed() || t.Failed() {
			g.Ended++
		}
		if t.Reworked {
			g.Reworked++
		}
		if t.Completed() && t.Observed {
			g.Completed++
			g.Tokens.add(t.TotalTokens)
		}
	}
	if g.Completed > 0 {
		g.BillablePerTask = float64(g.Tokens.Billable()) / float64(g.Completed)
		g.TokensPerTask = float64(g.Tokens.Total) / float64(g.Completed)
	}
	if g.Ended > 0 {
		g.ReworkRate = float64(g.Reworked) / float64(g.Ended)
	}
	rep.ByVerdict = verdicts.rows()
	return rep
}

// ReportJSON renders the report as indented JSON.
func ReportJSON(r Report) (string, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// PrintReport writes the human-readable report.
func PrintReport(w io.Writer, r Report) {
	window := fmt.Sprintf("last %d days", r.Days)
	if r.Days <= 0 {
		window = "all time"
	}
	fmt.Fprintf(w, "Routed spawns, %s\n\n", window)
	printGroupTable(w, r)
	fmt.Fprintln(w)
	if r.Routed.Spawns == 0 {
		fmt.Fprintf(w, "No routed spawn in this window.\n\n")
	}
	printVerdictRework(w, r)

	// Tokens only: the plan lens, tokens by where they ran, one row per harness.
	PrintRouting(w, r.Routing)

	printPermissionModes(w, r.DecisionsByPermissionMode)
	if len(r.Completion) > 0 {
		fmt.Fprintln(w, "\nCompletion measurable, by harness")
		for _, c := range r.Completion {
			fmt.Fprintf(w, "  %-12s %s\n", c.Harness, c.Describe())
		}
		fmt.Fprintln(w)
	}
	if r.Excluded > 0 {
		fmt.Fprintf(w, "%d spawn(s) left unrouted by a session pause, or logged as baseline by an older version, are not part of these figures.\n", r.Excluded)
	}
	fmt.Fprintln(w, "Billable tokens = input, output and cache write tokens of the same tasks.")
	fmt.Fprintln(w, "Cache reads are left out of them: they follow the context a child rereads, not the tier it ran on.")
	fmt.Fprintln(w, "A cheaper tier is not fewer tokens: the figures count tokens that ran, per harness, never summed.")
	fmt.Fprintln(w, "Completed tasks include the failed attempts they redid (respawn in the same session within 10 minutes). Prompt-free.")
}

// printGroupTable prints the routed row: spawns, ends, completed tasks,
// billable tokens per completed task and the rework rate.
func printGroupTable(w io.Writer, r Report) {
	fmt.Fprintf(w, "%-9s %7s %7s %10s %19s %9s\n", "group", "spawns", "ended", "completed", "billable/completed", "rework")
	g := r.Routed
	tokens := "-"
	if g.Completed > 0 {
		tokens = formatTokens(int64(g.BillablePerTask + 0.5))
	}
	fmt.Fprintf(w, "%-9s %7d %7d %10d %19s %9s\n", "routed", g.Spawns, g.Ended, g.Completed, tokens, reworkCell(g.Ended, g.ReworkRate))
}
