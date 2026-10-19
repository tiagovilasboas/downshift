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
	Days        int     // time window; <= 0 means all time
	CostPerUnit float64 // USD per normalised unit; 0 means show units only
	// Detail adds the full report (normalised units, honor, quota, usage, child
	// status, per-session cost, complexity breakdown) after the 3-line headline.
	Detail bool
	// ShowDCI displays the Code Intelligence (DCI) metrics section.
	ShowDCI bool
}

// PrintStats writes a human-readable stats summary to w.
func PrintStats(events []Event, opts StatsOptions, w io.Writer) {
	filtered := FilterByDays(events, opts.Days)
	s := Aggregate(filtered)
	normSaved, normFrac := s.NormSaved()

	label := fmt.Sprintf("Last %d days", opts.Days)
	if opts.Days <= 0 {
		label = "All time"
	}

	printHeadline(w, s, opts.Days)
	printTokensLine(w, s.TokensRerouted)
	fmt.Fprintf(w, "Estimated at list price, not an invoice. Measured against a control group: downshift report\n")
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

	if s.Total > 0 {
		if opts.CostPerUnit > 0 {
			// Dollar mode: multiply normalised units by the user-supplied rate.
			baseline := s.NormBaseline * opts.CostPerUnit
			routed := s.NormRouted * opts.CostPerUnit
			saved := normSaved * opts.CostPerUnit
			fmt.Fprintf(w, "Estimated baseline    $%10.2f\n", baseline)
			fmt.Fprintf(w, "Estimated routed      $%10.2f\n", routed)
			fmt.Fprintf(w, "Estimated savings     $%10.2f  (%4.1f%%)\n", saved, normFrac*100)
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "Rate: $%.4f / unit  (--cost-per-unit=%.4f)\n", opts.CostPerUnit, opts.CostPerUnit)
		} else {
			// Unit mode: dimensionless proxy, no dollar claim.
			fmt.Fprintf(w, "Normalised baseline   %8.1f units\n", s.NormBaseline)
			fmt.Fprintf(w, "Normalised routed     %8.1f units\n", s.NormRouted)
			fmt.Fprintf(w, "Normalised savings    %8.1f units  (%4.1f%%)\n", normSaved, normFrac*100)
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "Note: 1 unit = cost of one unrouted event. Not real dollars.\n")
			fmt.Fprintf(w, "For dollar figures: downshift stats --cost-per-unit=<USD-per-unit>\n")
		}
	}
	if s.Corrected > 0 {
		fmt.Fprintf(w, "Safety-held           %8d  (guardrail applied, see corrections)\n", s.Corrected)
	}
	if s.ActualCostEvents > 0 {
		fmt.Fprintf(w, "Observed token cost   $%10.2f  (%d events, catalog pricing)\n", s.ActualCostUSD, s.ActualCostEvents)
		printCostGroups(w, s.CostBySession, s.CostBySubagent)
	}
	if s.ObservedChildUsage == 0 {
		fmt.Fprintf(w, "Observed child tokens unobserved\n")
	} else {
		fmt.Fprintf(w, "Observed child tokens %12d  (%d usage records)\n", s.ObservedChildTokens, s.ObservedChildUsage)
	}
	if s.RealCostEvents > 0 {
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "Real provider cost (%d events with token usage)\n", s.RealCostEvents)
		fmt.Fprintf(w, "Real saved            $%10.2f  (catalog list-price counterfactual)\n", s.RealSavedUSD)
		fmt.Fprintf(w, "Real cost delta       $%10.2f  (baseline minus actual; negative means the router cost more)\n", s.RealCostDeltaUSD)
		fmt.Fprintf(w, "Real overspend        $%10.2f  (events that cost more than their baseline)\n", s.RealOverspendUSD)
		if s.RealCostInferred > 0 {
			fmt.Fprintf(w, "Inferred baseline     %8d  of %d events: baseline guessed from settings (BASELINE_FROM_SETTINGS), not requested\n", s.RealCostInferred, s.RealCostEvents)
		}
	} else if s.ActualCostEvents > 0 {
		fmt.Fprintf(w, "Savings comparison: no events with a known requested baseline yet.\n")
	} else {
		fmt.Fprintf(w, "Real cost: no events with token usage yet (wire the PostToolUse hook to populate them).\n")
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
		printScoreLine(w, s)
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

// printScoreLine reports the public-score capability path. It prints nothing
// when no decision carries a pick, so output without a score index is unchanged.
func printScoreLine(w io.Writer, s Stats) {
	if s.ScoreObserved == 0 {
		return
	}
	fmt.Fprintf(w, "Score pick differs    %d of %d decisions (%.0f%%)", s.ScoreDiffers, s.ScoreObserved, pct(s.ScoreDiffers, s.ScoreObserved))
	if s.ScoreRatioN > 0 {
		avg := s.ScoreRatioSum / float64(s.ScoreRatioN)
		switch {
		case avg < 1:
			fmt.Fprintf(w, ", list price %.0f%% lower on average", (1-avg)*100)
		case avg > 1:
			fmt.Fprintf(w, ", list price %.0f%% higher on average", (avg-1)*100)
		default:
			fmt.Fprintf(w, ", same list price on average")
		}
	}
	fmt.Fprintf(w, " (shadow unless DOWNSHIFT_SCORE_ROUTING=1)\n")
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
