// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Aggregate computes Stats from a slice of events.
//
// The decision/refutation index is built first (exported windows may arrive
// out of order); one pass over the events then runs the named steps in a fixed
// order; the whole-log tallies run last.
func Aggregate(events []Event) Stats {
	a := newAggregator()
	a.indexDecisions(events)
	for _, ev := range events {
		a.sumActualCost(ev)
		if a.countChildEnd(ev) {
			continue // how a child ended: an observation, not a decision or cost
		}
		if ev.Outcome == OutcomeResolved {
			continue // an observation about an earlier decision, not a cost or a decision
		}
		if ev.Outcome == OutcomeDCI || ev.DCI != nil {
			a.countDCI(ev)
			continue
		}
		if IsCostOnlyOutcome(ev.Outcome) {
			a.countCostOnly(ev)
			continue
		}
		a.countDecision(ev)
	}
	a.tallyLogWide(events)
	a.finalizeDCI()
	a.s.DuplicateToolUse = DuplicateToolUseCount(events)
	return a.s
}

// linkKey identifies a decision (or a record linked to one) within a harness.
type linkKey struct {
	harness, decision string
}

// aggregator accumulates Stats over one Aggregate call. Each method is one
// concern of the single pass.
type aggregator struct {
	s         Stats
	decisions map[linkKey]Event
	// refuted holds rewrites a linked "resolved" record says the harness did
	// not run: they are not applied and save nothing.
	refuted map[linkKey]bool
	// linked marks usage records already matched to a decision.
	linked       map[linkKey]bool
	dciLatencies []float64
}

func newAggregator() *aggregator {
	return &aggregator{
		s: Stats{
			ByComplexity:   make(map[string]int),
			ByEntry:        make(map[string]int),
			ChildStatus:    make(map[ChildStatus]int),
			CostBySession:  make(map[string]CostGroup),
			CostBySubagent: make(map[string]CostGroup),
			ReasonCodes:    make(map[string]int),
			ModelMix:       make(map[string]int),
			ByAgentType:    make(map[string]AgentTypeCount),
			DCI: DCIStats{
				AbstentionsByReason: make(map[string]int),
				ErrorsByCode:        make(map[string]int),
			},
		},
		decisions: make(map[linkKey]Event),
		refuted:   make(map[linkKey]bool),
		linked:    make(map[linkKey]bool),
	}
}

// indexDecisions builds the decision and refuted-rewrite indexes before any
// counting.
func (a *aggregator) indexDecisions(events []Event) {
	for _, ev := range events {
		if !IsCostOnlyOutcome(ev.Outcome) && !IsFailureOutcome(ev.Outcome) {
			a.decisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			a.refuted[linkKey{ev.Harness, ev.LinkedDecision}] = true
		}
	}
}

// sumActualCost adds provider-reported actual cost, and groups it by session
// and subagent for usage records.
func (a *aggregator) sumActualCost(ev Event) {
	if ev.Outcome == OutcomeResolved || ev.Outcome == OutcomeBaseline || ev.ActualCostUSD == nil {
		return
	}
	a.s.ActualCostEvents++
	a.s.ActualCostUSD += *ev.ActualCostUSD
	if ev.Outcome == OutcomeUsage {
		addGroupCost(a.s.CostBySession, ev.SessionID, *ev.ActualCostUSD)
		addGroupCost(a.s.CostBySubagent, ev.AgentHash, *ev.ActualCostUSD)
	}
}

// countChildEnd tallies how a child ended and reports whether ev was such a
// record.
func (a *aggregator) countChildEnd(ev Event) bool {
	if ev.Outcome == OutcomeChildEnded && ev.ChildStatus != "" {
		a.s.ChildStatus[ev.ChildStatus]++
		return true
	}
	return false
}

// countCostOnly handles post-hoc cost records ("usage") and control-group
// events ("baseline"). They are not decisions: they must not move Total,
// verdict counts, complexity counts, or normalised units. Usage records
// contribute real-dollar savings.
func (a *aggregator) countCostOnly(ev Event) {
	if ev.Outcome == OutcomeBaseline {
		a.s.Baseline++
		return
	}
	a.s.UsageRecords++
	key := linkKey{ev.Harness, ev.LinkedDecision}
	a.linkUsage(ev, key)
	// A refuted rewrite saved nothing; an older record may still be priced on
	// the model it asked for, so its delta is not credited.
	if ev.HasRealCost() && !a.refuted[key] {
		decision, found := a.decisions[key]
		a.addRealCost(RoutedCost(ev, decision, found), hasReason(decision.ReasonCodes, ReasonBaselineFromSettings))
	}
}

// linkUsage counts a usage record once per distinct subagent (or event) when
// it links to a known decision in the same session.
func (a *aggregator) linkUsage(ev Event, key linkKey) {
	identity := "event:" + DecisionKey(ev)
	if ev.AgentHash != "" {
		identity = "agent:" + ev.AgentHash
	}
	unique := linkKey{ev.Harness, identity}
	if decision, ok := a.decisions[key]; ok && ev.LinkedDecision != "" && !a.linked[unique] &&
		(ev.SessionID == "" || decision.SessionID == "" || ev.SessionID == decision.SessionID) {
		a.s.UsageLinked++
		a.linked[unique] = true
	}
}

// addRealCost credits the real-dollar delta of an event with token usage and
// notes when its baseline was inferred.
func (a *aggregator) addRealCost(ev Event, baselineFromSettings bool) {
	a.s.RealCostEvents++
	a.s.addRealDelta(ev)
	if ev.BaselineSource == BaselineInferred || baselineFromSettings {
		a.s.RealCostInferred++
	}
}

// countDecision tallies one routing decision across all of its dimensions.
func (a *aggregator) countDecision(ev Event) {
	a.s.Total++
	a.countAgentType(ev)
	a.s.ByEntry[NormalizeEntry(ev.Entry)]++
	if ev.PolicyFloor {
		a.s.PolicyFloor++
	}
	a.s.ByComplexity[ev.Complexity]++
	isRefuted := a.refuted[linkKey{ev.Harness, DecisionKey(ev)}]
	a.countModelMix(ev, isRefuted)
	a.countScoreShadow(ev)
	for _, code := range ev.ReasonCodes {
		a.s.ReasonCodes[code]++
	}
	a.countFrontier(ev, isRefuted)
	applied := AppliedRewrite(ev.Outcome, ev.Corrections) && !isRefuted
	a.countVerdict(ev, applied)
	a.sumNormalisedCost(ev, applied)
	// Real provider cost: only events with token usage contribute.
	if ev.HasRealCost() {
		a.addRealCost(ev, hasReason(ev.ReasonCodes, ReasonBaselineFromSettings))
	}
	// Self-correction: the verdict stands, the safe action applied.
	if len(ev.Corrections) > 0 {
		a.s.Corrected++
	}
}

func (a *aggregator) countAgentType(ev Event) {
	if ev.AgentType == "" {
		return
	}
	c := a.s.ByAgentType[ev.AgentType]
	c.Decisions++
	if ev.FromModel == "" || ev.FromModel == "unknown" {
		c.UnknownBaseline++
	}
	a.s.ByAgentType[ev.AgentType] = c
}

// countModelMix counts the model that actually ran.
func (a *aggregator) countModelMix(ev Event, isRefuted bool) {
	ran := ev.ToModel
	if isRefuted {
		ran = ev.FromModel // the harness kept the requested model
	}
	if ran != "" && ran != "unknown" {
		a.s.ModelMix[ShortModel(ran)]++
	}
}

// countScoreShadow tallies the score-routing shadow observation.
func (a *aggregator) countScoreShadow(ev Event) {
	if ev.ScoreWould == "" {
		return
	}
	a.s.ScoreObserved++
	if ev.ScoreDiffers {
		a.s.ScoreDiffers++
	}
	if ev.ScoreWouldCostRatio > 0 {
		a.s.ScoreRatioN++
		a.s.ScoreRatioSum += ev.ScoreWouldCostRatio
	}
}

// countFrontier counts frontier picks and the subset spent on doubt.
func (a *aggregator) countFrontier(ev Event, isRefuted bool) {
	if !strings.EqualFold(ev.Tier, core.TierFrontier.String()) {
		return
	}
	a.s.FrontierDecisions++
	if spentOnDoubt(ev) && !isRefuted {
		a.s.FrontierOnDoubt++
	}
}

func (a *aggregator) countVerdict(ev Event, applied bool) {
	switch {
	case ev.Verdict == "DOWNSHIFT" && applied:
		a.s.Downshifted++
	case ev.Verdict == "UPSHIFT" && applied:
		a.s.Upshifted++
	case ev.Verdict == "DOWNSHIFT" || ev.Verdict == "UPSHIFT":
		a.s.NotApplied++
	case ev.Verdict == "OK":
		a.s.OK++
	default:
		a.s.Unknown++
	}
}

// sumNormalisedCost: baseline = 1.0 per event; routed = 1 - savings. Only an
// applied downshift whose quota surface is available saves anything. unknown
// and held are not credited routes. This is a dimensionless proxy. Multiply by
// --cost-per-unit to get dollars.
func (a *aggregator) sumNormalisedCost(ev Event, applied bool) {
	a.s.NormBaseline += 1.0
	if ev.Verdict == "DOWNSHIFT" && applied && SavingsCredit(ev) && ev.EstimatedSavings > 0 && ev.EstimatedSavings < 1 {
		a.s.NormRouted += 1 - ev.EstimatedSavings
	} else if ev.Verdict == "UPSHIFT" && applied && ev.UpshiftCostRatio > 1 {
		// An applied upshift costs more than the baseline. Charge the
		// known price ratio; an unknown ratio stays neutral (1.0).
		a.s.NormRouted += ev.UpshiftCostRatio
	} else {
		a.s.NormRouted += 1.0
	}
}

// tallyLogWide runs the tallies that need the whole event list rather than one
// event at a time.
func (a *aggregator) tallyLogWide(events []Event) {
	a.s.RewriteShifted, a.s.RewriteHonored = CountInferredHonored(events)
	tallySpawnSurface(events, &a.s)
	a.s.ObservedChildTokens, a.s.ObservedChildUsage = ObservedChildTokens(events)
	a.s.RespawnAfterFailure = len(RespawnAfterFailure(events, respawnWindow))
}

// countDCI accumulates observed Code Intelligence metrics from an OutcomeDCI event.
func (a *aggregator) countDCI(ev Event) {
	if ev.DCI == nil {
		return
	}
	d := ev.DCI
	a.s.DCI.PromptsEvaluated++
	if d.InjectionApplied {
		a.s.DCI.InjectionsApplied++
		a.s.DCI.ContextBytesEmitted += int64(d.OutputBytes)
	} else if d.AbstentionReason != "" {
		a.s.DCI.AbstentionsByReason[d.AbstentionReason]++
	}
	if d.ErrorCode != "" {
		a.s.DCI.Errors++
		a.s.DCI.ErrorsByCode[d.ErrorCode]++
	}
	if d.TotalDurationMs >= 0 {
		a.dciLatencies = append(a.dciLatencies, float64(d.TotalDurationMs))
	}
}

// finalizeDCI computes summary averages and p50/p95 latencies for DCI.
func (a *aggregator) finalizeDCI() {
	if a.s.DCI.InjectionsApplied > 0 {
		a.s.DCI.AvgContextBytes = float64(a.s.DCI.ContextBytesEmitted) / float64(a.s.DCI.InjectionsApplied)
	}
	if len(a.dciLatencies) > 0 {
		sort.Float64s(a.dciLatencies)
		p50Idx := int(float64(len(a.dciLatencies)) * 0.50)
		if p50Idx >= len(a.dciLatencies) {
			p50Idx = len(a.dciLatencies) - 1
		}
		p95Idx := int(float64(len(a.dciLatencies)) * 0.95)
		if p95Idx >= len(a.dciLatencies) {
			p95Idx = len(a.dciLatencies) - 1
		}
		a.s.DCI.LatencyP50Ms = a.dciLatencies[p50Idx]
		a.s.DCI.LatencyP95Ms = a.dciLatencies[p95Idx]
	}
}
