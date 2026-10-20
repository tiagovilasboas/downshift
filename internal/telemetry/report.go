// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"sort"
)

// MinReportTasks is the least number of completed, priced tasks each group
// needs before the report states a savings figure.
const MinReportTasks = 20

const (
	bootstrapResamples = 2000
	bootstrapSeed      = 20261013
)

// GroupSummary is one group's side of the comparison.
type GroupSummary struct {
	Spawns int `json:"spawns"` // decisions in the group
	Ended  int `json:"ended"`  // spawns with an observed end: completed, error or interrupted
	// Completed counts completed tasks with a priced cost; only these enter
	// CostUSD and CostPerTaskUSD.
	Completed int `json:"completed_tasks"`
	// CostUSD is the cost of those tasks plus the failed attempts they redid.
	CostUSD        float64 `json:"cost_usd"`
	CostPerTaskUSD float64 `json:"cost_per_completed_task_usd"`
	// Tokens is the token count of the same tasks (priced usage records, the
	// failed attempts they redid included), by billing class; Tokens.Records is
	// the number of usage records. TokensPerTask is Tokens.Total over Completed.
	// It is the check that a cheaper model did not need more tokens.
	Tokens        TokenClasses `json:"tokens"`
	TokensPerTask float64      `json:"tokens_per_completed_task"`
	// Reworked counts failed spawns that were redone; ReworkRate is Reworked
	// over Ended.
	Reworked   int     `json:"reworked"`
	ReworkRate float64 `json:"rework_rate"`

	costs []float64
}

// Savings compares control with routed cost per completed task, in dollars
// (the API lens). Positive means routing was cheaper. The interval is a 95%
// percentile bootstrap with a fixed seed, so the same log always prints the
// same numbers. There is no percentage: a dollar share would hide the price
// table it rests on; the plan lens states the share in tokens.
type Savings struct {
	PerTaskUSD     float64 `json:"per_task_usd"`
	PerTaskLowUSD  float64 `json:"per_task_low_usd"`
	PerTaskHighUSD float64 `json:"per_task_high_usd"`
	Resamples      int     `json:"resamples"`
}

// Report is the routed-versus-control comparison. Prompt-free: counts, rates
// and catalog-priced dollars only.
type Report struct {
	Days            int          `json:"days"`
	Routed          GroupSummary `json:"routed"`
	Control         GroupSummary `json:"control"`
	Excluded        int          `json:"excluded_unrouted"` // DOWNSHIFT_NO_ROUTE or paused spawns
	ControlShareObs float64      `json:"control_share_observed"`
	MinTasks        int          `json:"min_tasks_per_group"`
	Enough          bool         `json:"enough_data"`
	Message         string       `json:"message,omitempty"`
	Savings         *Savings     `json:"savings,omitempty"`
	// Completion says, per harness, whether the end of a child is measurable.
	Completion []HarnessCompletion `json:"completion"`
	// PricesAsOf is the catalog date the dollars were priced at (API lens);
	// the caller sets it after RepriceUsage. Empty means as recorded.
	PricesAsOf string `json:"prices_as_of,omitempty"`
	// TokensSaved is the plan lens: tokens saved from the requested model in
	// the window, work and cache apart, with no price at all.
	TokensSaved TokenSaving `json:"tokens_saved"`
}

// BuildReport filters events to the last days (<= 0 means all time) and
// compares the routed and control groups.
func BuildReport(events []Event, days int) Report {
	rep := Report{Days: days, MinTasks: MinReportTasks}
	windowed := FilterByDays(events, days)
	rep.Completion = CompletionByHarness(windowed)
	rep.TokensSaved = TokenSavings(windowed).Total
	tasks := BuildTasks(windowed)
	for _, t := range tasks {
		var g *GroupSummary
		switch t.Group {
		case GroupRouted:
			g = &rep.Routed
		case GroupControl:
			g = &rep.Control
		default:
			rep.Excluded++
			continue
		}
		g.Spawns++
		if t.Completed() || t.Failed() {
			g.Ended++
		}
		if t.Reworked {
			g.Reworked++
		}
		if t.Completed() && t.Priced {
			g.Completed++
			g.CostUSD += t.TotalUSD
			g.Tokens.add(t.TotalTokens)
			g.costs = append(g.costs, t.TotalUSD)
		}
	}
	for _, g := range []*GroupSummary{&rep.Routed, &rep.Control} {
		if g.Completed > 0 {
			g.CostPerTaskUSD = g.CostUSD / float64(g.Completed)
			g.TokensPerTask = float64(g.Tokens.Total) / float64(g.Completed)
		}
		if g.Ended > 0 {
			g.ReworkRate = float64(g.Reworked) / float64(g.Ended)
		}
	}
	if n := rep.Routed.Spawns + rep.Control.Spawns; n > 0 {
		rep.ControlShareObs = float64(rep.Control.Spawns) / float64(n)
	}
	switch {
	case rep.Routed.Completed < MinReportTasks || rep.Control.Completed < MinReportTasks:
		rep.Message = fmt.Sprintf("not enough data: need at least %d completed tasks per group (routed %d, control %d)",
			MinReportTasks, rep.Routed.Completed, rep.Control.Completed)
	case rep.Control.CostPerTaskUSD <= 0:
		rep.Message = "not enough data: control cost per completed task is zero"
	default:
		rep.Enough = true
		rep.Savings = bootstrapSavings(rep.Control.costs, rep.Routed.costs)
	}
	return rep
}

func mean(xs []float64) float64 {
	var s float64
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func resampleMean(rng *rand.Rand, xs []float64) float64 {
	var s float64
	for range xs {
		s += xs[rng.Intn(len(xs))]
	}
	return s / float64(len(xs))
}

func percentile(sorted []float64, p float64) float64 {
	i := int(p * float64(len(sorted)))
	if i >= len(sorted) {
		i = len(sorted) - 1
	}
	return sorted[i]
}

// bootstrapSavings resamples both groups independently with a fixed seed.
func bootstrapSavings(control, routed []float64) *Savings {
	mc, mr := mean(control), mean(routed)
	s := &Savings{PerTaskUSD: mc - mr, Resamples: bootstrapResamples}
	rng := rand.New(rand.NewSource(bootstrapSeed))
	deltas := make([]float64, 0, bootstrapResamples)
	for i := 0; i < bootstrapResamples; i++ {
		c, r := resampleMean(rng, control), resampleMean(rng, routed)
		deltas = append(deltas, c-r)
	}
	sort.Float64s(deltas)
	s.PerTaskLowUSD, s.PerTaskHighUSD = percentile(deltas, 0.025), percentile(deltas, 0.975)
	return s
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
	fmt.Fprintf(w, "Routed vs control, %s\n\n", window)
	fmt.Fprintf(w, "%-9s %7s %7s %10s %17s %9s\n", "group", "spawns", "ended", "completed", "tokens/completed", "rework")
	for _, row := range []struct {
		name string
		g    GroupSummary
	}{{"routed", r.Routed}, {"control", r.Control}} {
		tokens := "-"
		if row.g.Completed > 0 {
			tokens = formatTokens(int64(row.g.TokensPerTask + 0.5))
		}
		rework := "-"
		if row.g.Ended > 0 {
			rework = fmt.Sprintf("%.1f%%", row.g.ReworkRate*100)
		}
		fmt.Fprintf(w, "%-9s %7d %7d %10d %17s %9s\n", row.name, row.g.Spawns, row.g.Ended, row.g.Completed, tokens, rework)
	}
	fmt.Fprintln(w)
	if r.Savings == nil {
		fmt.Fprintf(w, "%s.\n\n", r.Message)
	}

	// Tokens only: the plan lens, tokens by where they ran. No dollars.
	fmt.Fprintln(w, "Plan lens (tokens only)")
	if line := r.TokensSaved.Line(); line != "" {
		fmt.Fprintf(w, "  %s\n", line)
		if r.TokensSaved.WorkSaved+r.TokensSaved.CacheSaved == 0 {
			fmt.Fprintln(w, "  no tokens saved in this window: every observed spawn ran on the requested model")
		}
	} else {
		fmt.Fprintln(w, "  tokens not observed in this window")
	}
	if r.Savings != nil {
		printTokenCheck(w, r)
	}

	if len(r.Completion) > 0 {
		fmt.Fprintln(w, "\nCompletion measurable, by harness")
		for _, c := range r.Completion {
			fmt.Fprintf(w, "  %-12s %s\n", c.Harness, c.Describe())
		}
		fmt.Fprintln(w)
	}
	if r.Excluded > 0 {
		fmt.Fprintf(w, "%d spawn(s) left unrouted by DOWNSHIFT_NO_ROUTE or a session pause are not part of the comparison.\n", r.Excluded)
	}
	fmt.Fprintln(w, "Tokens per completed task = input, output and cache tokens of the same tasks; a cheaper model should not need more of them.")
	fmt.Fprintln(w, "Tokens saved = tokens that ran on a cheaper model than requested; work (input+output) and cache are counted apart.")
	fmt.Fprintln(w, "Completed tasks include the failed attempts they redid (respawn in the same session within 10 minutes). Prompt-free.")
}

// printTokenCheck compares tokens per completed task between the groups, once
// the report has enough data to state a saving. Tokens are counts, not a
// price: routed using more tokens than control is the signal to read.
func printTokenCheck(w io.Writer, r Report) {
	c, d := r.Control.TokensPerTask, r.Routed.TokensPerTask
	if c <= 0 || r.Routed.Completed == 0 {
		return
	}
	change := (d - c) / c * 100
	fmt.Fprintf(w, "Tokens per completed task   routed %s vs control %s  (%+.1f%%; not an interval)\n",
		formatTokens(int64(d+0.5)), formatTokens(int64(c+0.5)), change)
}
