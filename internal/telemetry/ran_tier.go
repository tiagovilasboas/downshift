// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// RanTier is the catalog tier of the model a spawn actually ran on: the
// applied model, not the classifier's recommendation (Event.Tier).
type RanTier string

// The ran-on tiers. RanUnknown means no model could be placed in the catalog.
const (
	RanSmall    RanTier = "small"
	RanMid      RanTier = "mid"
	RanFrontier RanTier = "frontier"
	RanUnknown  RanTier = "unknown"
)

// SpawnRef names one spawn: its harness and decision key (TaskRecord.Key).
type SpawnRef struct{ Harness, Key string }

// RanTiers returns the tier each decision's child ran on. The model resolves
// as for RoutingTokens (tokenModels): a usage record is the observation and
// wins; otherwise the applied model of the decision (the final model of an
// applied rewrite, the requested one of an allow, a hold or a rewrite a
// resolved record refuted). A resumed child with several usage records takes
// its highest tier. A model the catalog does not rank, or no catalog, is
// RanUnknown, never a guess. A rewrite the harness never confirmed is assumed
// honored, as everywhere else in the report.
func RanTiers(events []Event) map[SpawnRef]RanTier {
	a := newAggregator()
	a.indexDecisions(events)
	usage := map[linkKey][]Event{}
	for _, ev := range events {
		if ev.Outcome == OutcomeUsage && ev.LinkedDecision != "" {
			k := linkKey{ev.Harness, ev.LinkedDecision}
			usage[k] = append(usage[k], ev)
		}
	}
	out := map[SpawnRef]RanTier{}
	for _, d := range events {
		if !isTaskDecision(d) {
			continue
		}
		k := linkKey{d.Harness, DecisionKey(d)}
		records := usage[k]
		if len(records) == 0 {
			records = []Event{{}}
		}
		best := RanUnknown
		_, isRefuted := a.refuted[k]
		for _, u := range records {
			_, ran := tokenModels(u, d, true, isRefuted)
			best = higherRan(best, ranTierOf(d.Harness, ran))
		}
		out[SpawnRef{d.Harness, k.decision}] = best
	}
	return out
}

func ranTierOf(harness, model string) RanTier {
	if DirectionCatalog == nil || !knownModel(model) {
		return RanUnknown
	}
	var rank int
	var ok bool
	if IsGatewayModel(model) {
		rank, ok = DirectionCatalog.GatewayTier(model)
	} else {
		_, rank, ok = DirectionCatalog.Lookup(harness, model)
	}
	if !ok {
		return RanUnknown
	}
	switch rank {
	case TierRankSmall:
		return RanSmall
	case TierRankMid:
		return RanMid
	case TierRankFrontier:
		return RanFrontier
	}
	return RanUnknown
}

func ranRank(t RanTier) int {
	switch t {
	case RanSmall:
		return 1
	case RanMid:
		return 2
	case RanFrontier:
		return 3
	}
	return 0
}

func higherRan(a, b RanTier) RanTier {
	if ranRank(b) > ranRank(a) {
		return b
	}
	return a
}
