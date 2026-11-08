// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
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
		if a.countChildEnd(ev) {
			continue // how a child ended: an observation, not a decision or cost
		}
		if ev.Outcome == OutcomeWorkerEnded || ev.Outcome == OutcomeMCPTool {
			continue // a delegated worker's end: tokens for the worker budget, not a routed spawn
		}
		if ev.Outcome == OutcomeResolved {
			continue // an observation about an earlier decision, not a cost or a decision
		}
		if ev.Outcome == OutcomeDCI {
			continue // a line left by the removed code intelligence: not a decision
		}
		if IsCostOnlyOutcome(ev.Outcome) {
			a.countCostOnly(ev)
			continue
		}
		a.countDecision(ev)
	}
	a.tallyLogWide(events)
	a.s.Routing = RoutingTokens(events)
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
	// not run: they are not applied and save nothing. The value is the model
	// that record reports ("" when it names none).
	refuted map[linkKey]string
	// unrouted holds paused decisions and baseline lines older binaries wrote
	// (outcome baseline): spawns the router left alone. They are not in decisions.
	unrouted map[linkKey]bool
	// linked marks usage records already matched to a decision.
	linked map[linkKey]bool
}

func newAggregator() *aggregator {
	return &aggregator{
		s: Stats{
			ByComplexity: make(map[string]int),
			ByEntry:      make(map[string]int),
			ChildStatus:  make(map[ChildStatus]int),
			ReasonCodes:  make(map[string]int),
			ModelMix:     make(map[string]int),
			ByAgentType:  make(map[string]AgentTypeCount),
		},
		decisions: make(map[linkKey]Event),
		refuted:   make(map[linkKey]string),
		unrouted:  make(map[linkKey]bool),
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
		if ev.Outcome == OutcomeBaseline {
			a.unrouted[linkKey{ev.Harness, DecisionKey(ev)}] = true
		}
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			a.refuted[linkKey{ev.Harness, ev.LinkedDecision}] = ev.ToModel
		}
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

// countCostOnly handles post-hoc cost records ("usage") and unrouted
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

// countDecision tallies one routing decision across all of its dimensions.
func (a *aggregator) countDecision(ev Event) {
	a.s.Total++
	a.countAgentType(ev)
	a.s.ByEntry[NormalizeEntry(ev.Entry)]++
	if ev.PolicyFloor {
		a.s.PolicyFloor++
	}
	a.s.ByComplexity[ev.Complexity]++
	served, isRefuted := a.refuted[linkKey{ev.Harness, DecisionKey(ev)}]
	a.countModelMix(ev, isRefuted, served)
	a.countGatewayShadow(ev)
	for _, code := range ev.ReasonCodes {
		a.s.ReasonCodes[code]++
	}
	a.countFrontier(ev, isRefuted)
	applied := AppliedRouting(ev) && !isRefuted
	a.countVerdict(ev, applied)
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

// countModelMix counts the model that actually ran. For a refuted rewrite that
// is the model the resolved record reports, which another hook on the same call
// may have set; the requested model only when the record names none.
func (a *aggregator) countModelMix(ev Event, isRefuted bool, served string) {
	ran := ev.ToModel
	if isRefuted {
		ran = ranOnRefute(ev, served)
	}
	if ran != "" && ran != "unknown" {
		a.s.ModelMix[ShortModel(ran)]++
	}
}

// countGatewayShadow tallies the OpenRouter allowlist shadow observation.
func (a *aggregator) countGatewayShadow(ev Event) {
	if ev.GatewayWould == "" {
		return
	}
	a.s.GatewayObserved++
	if a.s.GatewaySlugs == nil {
		a.s.GatewaySlugs = make(map[string]int)
	}
	a.s.GatewaySlugs[ev.GatewayWould]++
}

// countFrontier counts frontier picks and the subset spent on doubt.
func (a *aggregator) countFrontier(ev Event, isRefuted bool) {
	if !strings.EqualFold(ev.Tier, TierFrontier) {
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

// tallyLogWide runs the tallies that need the whole event list rather than one
// event at a time.
func (a *aggregator) tallyLogWide(events []Event) {
	a.s.RewriteShifted, a.s.RewriteHonored = CountInferredHonored(events)
	tallySpawnSurface(events, &a.s)
	a.s.ObservedChildTokens, a.s.ObservedChildUsage = ObservedChildTokens(events)
	a.s.RespawnAfterFailure = len(RespawnAfterFailure(events, respawnWindow))
}
