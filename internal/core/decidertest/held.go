// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decidertest

import "github.com/tiagovilasboas/downshift/internal/core"

// Unconfident returns a Decider that routes to tier t like Tier but without
// confidence and with no guardrail correction: the decision a classifier
// makes on doubt when nothing else held it.
func Unconfident(t core.Tier) core.Decider {
	return core.DeciderFunc(func(req core.Request) core.Decision {
		d := Tier(t).Decide(req)
		d.Confident = false
		return d
	})
}

// Held returns a Decider that routes to tier t but holds the move: the
// classified verdict stays, the safe verdict is OK (keep the requested
// model), and Corrections names a guardrail rule. An adapter that obeys the
// hold leaves the spawn unchanged. The hold is unconfident, as a real one is.
func Held(t core.Tier) core.Decider {
	return core.DeciderFunc(func(req core.Request) core.Decision {
		d := Tier(t).Decide(req)
		d.Confident = false
		d.SafeVerdict = core.VerdictOK
		d.Corrections = []string{core.RuleUnconfidentDowngrade}
		return d
	})
}

// Mark returns a Decider that runs d and then lets f adjust the decision, for
// a contract test that needs one more fact on it (a flag, a guard).
func Mark(d core.Decider, f func(*core.Decision)) core.Decider {
	return core.DeciderFunc(func(req core.Request) core.Decision {
		out := d.Decide(req)
		f(&out)
		return out
	})
}

// When returns a Decider that asks yes for a request that match accepts and no
// for every other request, so one test can pin different decisions per spawn.
func When(match func(core.Request) bool, yes, no core.Decider) core.Decider {
	return core.DeciderFunc(func(req core.Request) core.Decision {
		if match(req) {
			return yes.Decide(req)
		}
		return no.Decide(req)
	})
}
