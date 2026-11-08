// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
)

// The class steps read the classifier class of the task (owner request
// 2026-11-01: route by task complexity as Ruflo does, and apply it). Moving
// down is on by default. Moving up is advice unless the policy asks for it:
//   - down (classChoice, precedence 5): a decisive TRIVIAL or SIMPLE class with
//     no risk category moves the spawn to small, unless a spawn Downshift moved
//     down in the same session just failed;
//   - up (raiseOrAdvise, after the guard): a decisive COMPLEX task the
//     orchestrator asked to run on small is moved to mid when
//     routing.raise_complex is true, and otherwise carries advice the adapter
//     hands to the orchestrator. It never goes past mid.

// classChoice is precedence 5: the tier the class asks for, taken only to move
// down. MEDIUM keeps: on the eval sets a decisive MEDIUM call still landed on
// frontier work.
func classChoice(in Input, requested contract.Tier) (choice, bool) {
	if !in.Policy.ClassRouting() || !in.Decisive || len(in.Risk) > 0 {
		return choice{}, false
	}
	if in.Class != "TRIVIAL" && in.Class != "SIMPLE" {
		return choice{}, false
	}
	if !requested.Valid() || rank(contract.TierSmall) >= rank(requested) {
		return choice{}, false
	}
	if in.RecentFailure {
		return choice{verdict: contract.VerdictKept, reason: contract.ReasonHeldAfterFailure}, true
	}
	return choice{verdict: contract.VerdictClass, tier: contract.TierSmall}, true
}

// raiseOrAdvise handles a decisive COMPLEX task the orchestrator asked to run
// on small, when no earlier step decided it (no pin, profile, rule or hint).
// It always sets the advice. With routing.raise_complex it also moves one tier
// up, to mid; the adapter writes that only when the session can run the mid
// model (a session inferred from the requested model holds only the tiers at
// or below it), and otherwise hands the orchestrator the advice instead.
func raiseOrAdvise(in Input, d core.Decision, n contract.Note) (core.Decision, contract.Note) {
	if !in.Policy.ClassRouting() || !in.Decisive || in.Class != "COMPLEX" || in.Request.Pinned {
		return d, n
	}
	// A declared profile or rule that chose small reads as kept too (same tier):
	// it decided, so neither the raise nor the advice overrides it.
	if n.Verdict != contract.VerdictKept || n.Profile != "" || n.RuleID != "" ||
		n.TierFrom != contract.TierSmall || n.TierTo != contract.TierSmall {
		return d, n
	}
	// The orchestrator's own tier hint is its choice: advice only, never a raise.
	d.Advice = core.AdviceComplexOnSmall
	if in.Policy.ClassRaise() && in.Catalog != nil && !in.Request.Hint.Tier.Valid() {
		target := targetModel(in, contract.TierMid)
		harness := string(in.Request.Harness)
		if target.ID != "" && !in.Catalog.IsExplicitOnly(harness, target.ID) && in.Catalog.IsExactID(harness, target.ID) {
			d.Tier, d.Model, d.Effort = target.Tier, target, core.EffortFor(target.Tier)
			d.Verdict, d.SafeVerdict = core.VerdictUpshift, core.VerdictUpshift
			d.Reason = ""
			n.Verdict, n.Reason, n.TierTo = contract.VerdictClassUp, contract.ReasonNone, contract.TierMid
			return d, n
		}
	}
	return d, n
}
