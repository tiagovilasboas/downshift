// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
)

// tallySpawnSurface counts observed honor and usage from linked port records,
// and quota from each decision's own status. It does not treat a later spawn
// or rewrite_emitted as honor.
func tallySpawnSurface(events []Event, s *Stats) {
	honor, usage := ObservedLinks(events)
	refuted := RefutedRewrites(events)
	for _, ev := range events {
		if ev.Harness == "" || IsCostOnlyOutcome(ev.Outcome) || IsFailureOutcome(ev.Outcome) {
			continue
		}
		surface := SurfaceFor(ev, honor[SurfaceKey(ev)], usage[SurfaceKey(ev)])
		// Honor is asked of a rewrite only: a spawn the hook left alone has no
		// written model to confirm, so counting it "unobserved" buried the few
		// real rewrites under every unchanged spawn.
		if isShift(ev) {
			_, isRefuted := refuted[SurfaceKey(ev)]
			switch {
			case surface.Honor == HonorObserved:
				s.HonorObserved++
			case isRefuted:
				s.HonorRefuted++
			default:
				s.HonorUnobserved++
			}
		}
		switch surface.Usage {
		case UsageObserved:
			s.UsageObserved++
		default:
			s.UsageUnobserved++
		}
		switch surface.Quota {
		case QuotaAvailable:
			s.QuotaAvailable++
		case QuotaHeld:
			s.QuotaHeld++
		default:
			s.QuotaUnknown++
		}
	}
}

// spentOnDoubt reports whether a frontier decision actually wrote the model on
// an unconfident classification with no deterministic lift. A decision a
// guardrail held, or one that left the spawn unchanged, spent nothing and is not
// counted: the alarm must only fire for money that was really spent.
func spentOnDoubt(ev Event) bool {
	return strings.EqualFold(ev.Tier, TierFrontier) &&
		ev.Outcome == OutcomeRewriteEmitted && AppliedRewrite(ev.Outcome, ev.Corrections) &&
		frontierOnDoubt(ev.ReasonCodes)
}

// frontierOnDoubt reports whether a decision's reason codes show an unconfident
// classification that no deterministic rule lifted.
func frontierOnDoubt(codes []string) bool {
	unconfident := false
	for _, c := range codes {
		switch c {
		case ReasonClassifierUnconfident:
			unconfident = true
		case ReasonRiskFloor, ReasonNBUpshiftApplied, ReasonTierFloor:
			return false
		}
	}
	return unconfident
}
