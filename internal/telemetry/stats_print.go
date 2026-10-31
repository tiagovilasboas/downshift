// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"sort"
)

// StatsOptions controls PrintStats output.
type StatsOptions struct {
	Days int // time window; <= 0 means all time
	// Detail adds the full report (honor, quota, usage, child status,
	// complexity breakdown) after the 3-line headline.
	Detail bool
	// ShowDCI displays the Code Intelligence (DCI) metrics section.
	ShowDCI bool
}

// PrintStats writes a human-readable stats summary to w.
func PrintStats(events []Event, opts StatsOptions, w io.Writer) {
	filtered := FilterByDays(events, opts.Days)
	s := Aggregate(filtered)

	label := fmt.Sprintf("Last %d days", opts.Days)
	if opts.Days <= 0 {
		label = "All time"
	}

	printHeadline(w, s, TokenSavings(filtered), opts.Days)
	fmt.Fprintf(w, "Tokens saved = ran on a cheaper model than requested. The routed-versus-control comparison: downshift report\n")
	if !opts.Detail && !opts.ShowDCI {
		fmt.Fprintf(w, "\nFull report: downshift stats --detail\n")
		return
	}
	if opts.ShowDCI && !opts.Detail {
		printDCISection(w, s.DCI)
		return
	}
	fmt.Fprintf(w, "\n%s\n", label)
	fmt.Fprintf(w, "─────────────────────────────────────\n")
	fmt.Fprintf(w, "Subagent decisions    %8d\n", s.Total)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Downshifted         %8d  %5.1f%%  (rewrite applied)\n", s.Downshifted, pct(s.Downshifted, s.Total))
	fmt.Fprintf(w, "  Upshifted           %8d  %5.1f%%  (rewrite applied)\n", s.Upshifted, pct(s.Upshifted, s.Total))
	fmt.Fprintf(w, "  Not applied         %8d  %5.1f%%  (classified shift, spawn unchanged: allow/held/blocked)\n", s.NotApplied, pct(s.NotApplied, s.Total))
	fmt.Fprintf(w, "  Unchanged (OK)      %8d  %5.1f%%\n", s.OK, pct(s.OK, s.Total))
	fmt.Fprintf(w, "  Unknown             %8d  %5.1f%%\n", s.Unknown, pct(s.Unknown, s.Total))
	if s.PolicyFloor > 0 {
		fmt.Fprintf(w, "  Raised by project floor %6d  %5.1f%%  (.downshift/policy.json)\n", s.PolicyFloor, pct(s.PolicyFloor, s.Total))
	}
	printChildStatus(w, s.ChildStatus)
	printAgentTypes(w, s.ByAgentType)
	if s.RespawnAfterFailure > 0 {
		fmt.Fprintf(w, "  Respawned after a failure  %5d  (hint only: may be unrelated work)\n", s.RespawnAfterFailure)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "By complexity\n")
	for _, cls := range []string{"TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"} {
		n := s.ByComplexity[cls]
		fmt.Fprintf(w, "  %-8s            %8d  %5.1f%%\n", cls, n, pct(n, s.Total))
	}
	fmt.Fprintf(w, "\n")

	if s.Corrected > 0 {
		fmt.Fprintf(w, "Safety-held           %8d  (guardrail applied, see corrections)\n", s.Corrected)
	}
	if s.ObservedChildUsage == 0 {
		fmt.Fprintf(w, "Observed child tokens unobserved\n")
	} else {
		fmt.Fprintf(w, "Observed child tokens %12d  (%d usage records)\n", s.ObservedChildTokens, s.ObservedChildUsage)
	}
	printTokensDetail(w, s.TokensRerouted)
	if s.UsageRecords > 0 {
		fmt.Fprintf(w, "Usage records        %8d  (all post-hoc usage records)\n", s.UsageRecords)
		fmt.Fprintf(w, "Usage linked          %8d  (distinct usage matched to decisions in this window)\n", s.UsageLinked)
	}
	if s.Baseline > 0 {
		fmt.Fprintf(w, "Baseline (no-route)   %8d  (control group, excluded from rates)\n", s.Baseline)
	}
	if s.Total > 0 {
		fmt.Fprintf(w, "Honor                 observed %d  unobserved %d  (applied rewrites only)\n", s.HonorObserved, s.HonorUnobserved)
		fmt.Fprintf(w, "Quota                 available %d  held %d  unknown %d\n", s.QuotaAvailable, s.QuotaHeld, s.QuotaUnknown)
		fmt.Fprintf(w, "Usage                 observed %d  unobserved %d\n", s.UsageObserved, s.UsageUnobserved)
		fmt.Fprintf(w, "Frontier decisions    %d  (rewritten on doubt, no deterministic lift: %d)\n", s.FrontierDecisions, s.FrontierOnDoubt)
		printGatewayLine(w, s)
		if n := s.ReasonCodes[ReasonSemanticBoost]; n > 0 {
			fmt.Fprintf(w, "Semantic boost        %d of %d decisions (%.0f%%) were raised by the embedder, not a rule\n", n, s.Total, pct(n, s.Total))
		}
		if n := s.ReasonCodes[ReasonUnsupportedLanguage]; n > 0 {
			fmt.Fprintf(w, "Language guard        %d of %d decisions (%.0f%%) lacked English/Portuguese rule coverage (%s)\n", n, s.Total, pct(n, s.Total), ReasonUnsupportedLanguage)
		}
		if s.FrontierOnDoubt > 0 {
			fmt.Fprintf(w, "WARNING: %d frontier rewrite(s) were applied on an unconfident classification with no deterministic lift; check for silent overspend.\n", s.FrontierOnDoubt)
		}
	}
	if s.DCI.PromptsEvaluated > 0 || opts.ShowDCI {
		printDCISection(w, s.DCI)
	}
	fmt.Fprintf(w, "─────────────────────────────────────\n")
}

// printDCISection writes the Code Intelligence (DCI) telemetry section.
// In accordance with Downshift's epistemic invariants, all metrics are strictly
// observed; no synthetic token counts or fictitious dollar savings are computed.
func printDCISection(w io.Writer, dci DCIStats) {
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "Code Intelligence (DCI)\n")
	if dci.PromptsEvaluated == 0 {
		fmt.Fprintf(w, "  No DCI prompt evaluations recorded in this window.\n")
		return
	}
	injPct := pct(dci.InjectionsApplied, dci.PromptsEvaluated)
	abstained := dci.PromptsEvaluated - dci.InjectionsApplied
	absPct := pct(abstained, dci.PromptsEvaluated)

	fmt.Fprintf(w, "  Prompts evaluated     %8d\n", dci.PromptsEvaluated)
	fmt.Fprintf(w, "  Injections applied    %8d  %5.1f%%  (context injected into harness)\n", dci.InjectionsApplied, injPct)
	fmt.Fprintf(w, "  Abstentions           %8d  %5.1f%%  (conversational / low relevance)\n", abstained, absPct)
	if len(dci.AbstentionsByReason) > 0 {
		keys := make([]string, 0, len(dci.AbstentionsByReason))
		for k := range dci.AbstentionsByReason {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, r := range keys {
			fmt.Fprintf(w, "    %-18s  %6d\n", r, dci.AbstentionsByReason[r])
		}
	}
	fmt.Fprintf(w, "  Context emitted       %8d bytes", dci.ContextBytesEmitted)
	if dci.InjectionsApplied > 0 {
		fmt.Fprintf(w, "  (avg %.0f bytes/injection)", dci.AvgContextBytes)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Observed latency      p50 %.1fms   p95 %.1fms\n", dci.LatencyP50Ms, dci.LatencyP95Ms)
	if dci.Errors > 0 {
		fmt.Fprintf(w, "  Errors (fail-open)    %8d\n", dci.Errors)
		for code, count := range dci.ErrorsByCode {
			fmt.Fprintf(w, "    error:%-12s  %6d\n", code, count)
		}
	}
}

// printGatewayLine reports the OpenRouter allowlist shadow. It prints nothing
// unless at least one decision would have been served by an allowlisted row.
// The slug shown is the most frequent one; ties break on the slug.
func printGatewayLine(w io.Writer, s Stats) {
	if s.GatewayObserved == 0 {
		return
	}
	top, topN := "", 0
	for slug, n := range s.GatewaySlugs {
		if n > topN || (n == topN && slug < top) {
			top, topN = slug, n
		}
	}
	fmt.Fprintf(w, "shadow: would route %d of %d spawns to %s", s.GatewayObserved, s.Total, top)
	if len(s.GatewaySlugs) > 1 {
		fmt.Fprintf(w, " (most frequent of %d slugs)", len(s.GatewaySlugs))
	}
	fmt.Fprintf(w, "\n")
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

// childStatusOrder is the fixed print order for "How children ended".
var childStatusOrder = []ChildStatus{ChildCompleted, ChildLaunched, ChildInterrupted, ChildError, ChildStopped, ChildUnknown}

// printChildStatus writes the "How children ended" section. It prints nothing
// when no child_ended events were counted, so output without them is unchanged.
func printChildStatus(w io.Writer, counts map[ChildStatus]int) {
	sum := 0
	for _, n := range counts {
		sum += n
	}
	if sum == 0 {
		return
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "How children ended\n")
	for _, st := range childStatusOrder {
		if n := counts[st]; n > 0 {
			fmt.Fprintf(w, "  %-11s         %8d\n", st, n)
		}
	}
	fmt.Fprintf(w, "  Note: launched is a background start, not a success.\n")
}
