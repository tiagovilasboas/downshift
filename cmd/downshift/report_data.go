// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"sort"

	"github.com/tiagovilasboas/downshift/internal/accounting"
)

// reportData is the report with no presentation: the header, one summary row
// per harness and the data of each section a tab shows. Every figure is per
// harness; nothing is summed across harnesses. Plain data: text, JSON,
// markdown and the styled report read it, none of them formats it here.
type reportData struct {
	Header     reportHeader
	Harnesses  []harnessSummary
	Savings    savingsSection
	Routing    routingSection
	Efficiency accounting.Efficiency
	Quota      []harnessQuota
}

// reportHeader names the window and the Compactor's own totals over it.
type reportHeader struct {
	Scope     accounting.Scope
	Label     string
	Days      int
	Commands  int64
	Compacted int64
}

// harnessSummary is the first-screen row of one harness. Observed flags say
// "unobserved", which is not the same as zero.
type harnessSummary struct {
	Harness string
	// TokensSaved is the Compactor's estimate (bytes / 4).
	TokensSaved int64
	// RoutingObserved is false when no spawn could be placed against the
	// session model; RoutedSharePct and Billable are then meaningless.
	RoutingObserved bool
	RoutedSharePct  float64
	Billable        int64
	// RewrittenSpawns and RewrittenBillable are the part of the below-session
	// figure that an applied Downshift rewrite caused.
	RewrittenSpawns   int
	RewrittenBillable int64
	// Premium preserved: NonFrontier of TierKnown completed tasks.
	PremiumObserved bool
	NonFrontier     int
	TierKnown       int
	PremiumPct      float64
	// Rework: Reworked of Ended routed tasks.
	Reworked  int
	Ended     int
	ReworkPct float64
}

// savingsSection is the Compactor per harness: the tokens avoided, then the
// detail by family and by why a command was not compacted.
type savingsSection struct {
	ByHarness []compactionRow
	Detail    accounting.CompactionDetail
}

// routingSection is the routing note and the routing tokens per harness.
type routingSection struct {
	Why       accounting.RoutingWhy
	ByHarness []accounting.HarnessTokens
}

// harnessQuota is the quota one harness has used.
type harnessQuota struct {
	Harness string
	Quota   accounting.QuotaUsed
}

// buildReportData folds the report's figures into reportData. Pure: it only
// rearranges what runReport already computed, for the window of scope.
func buildReportData(d reportOut, scope accounting.Scope) reportData {
	out := reportData{
		Header: reportHeader{
			Scope: scope, Label: scope.Label(), Days: scope.Days,
			Commands: d.Numbers.Compactor.Commands, Compacted: d.Numbers.Compactor.Compacted,
		},
		Savings:    savingsSection{ByHarness: d.CompactionByHarness, Detail: d.CompactionDetail},
		Routing:    routingSection{Why: d.WhySpawnsChanged, ByHarness: d.TokensByHarness},
		Efficiency: d.Efficiency,
		Quota:      []harnessQuota{},
	}
	tokens := map[string]accounting.HarnessTokens{}
	eff := map[string]accounting.HarnessEfficiency{}
	names := map[string]bool{}
	for _, t := range d.TokensByHarness {
		tokens[t.Harness], names[t.Harness] = t, true
		out.Quota = append(out.Quota, harnessQuota{Harness: t.Harness, Quota: t.Quota})
	}
	for _, e := range d.Efficiency.Harnesses {
		eff[e.Harness], names[e.Harness] = e, true
	}
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	out.Harnesses = make([]harnessSummary, 0, len(sorted))
	for _, n := range sorted {
		out.Harnesses = append(out.Harnesses, summaryOf(n, tokens[n], eff[n]))
	}
	return out
}

// summaryOf joins the routing-token row and the efficiency row of a harness;
// a missing side stays at its zero value with its Observed flag false.
func summaryOf(name string, t accounting.HarnessTokens, e accounting.HarnessEfficiency) harnessSummary {
	return harnessSummary{
		Harness: name, TokensSaved: t.CompactorAvoided,
		RoutingObserved: t.RoutingObserved, RoutedSharePct: t.Routing.BelowShare(), Billable: t.Routing.Billable,
		RewrittenSpawns: t.Routing.RewrittenSpawns, RewrittenBillable: t.Routing.RewrittenBillable,
		PremiumObserved: e.PremiumObserved, NonFrontier: e.NonFrontier, TierKnown: e.TierKnown, PremiumPct: e.PremiumPreservedPct,
		Reworked: e.Reworked, Ended: e.Ended, ReworkPct: 100 * e.ReworkRatio,
	}
}
