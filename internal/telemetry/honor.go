// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

// CountInferredHonored counts applied model rewrites that later look honored.
// The result is an inference, not proof: Downshift cannot see the model a
// child actually ran on, only the model a later spawn asked for.
//
// A shift is a rewrite the hook actually wrote: outcome rewrite_emitted, no
// guardrail correction, and requested_model != final_model. Allow, blocked,
// held and effort-only events never count as shifts. A shift counts as
// (inferred) honored when rewrite_honored is true on the event, or when a
// later decision event (any outcome) in the same harness + hashed session
// arrives with requested_model equal to that final_model.
//
// A later "resolved" record that links to the shift and reports a matching
// model also counts, and needs no later spawn. Events without a session id
// cannot be inferred. The log is not rewritten.
func CountInferredHonored(events []Event) (shifted, honored int) {
	// A "resolved" record is the harness reporting the model it chose for the
	// spawn. When it matches the written model it is direct observation.
	observed := make(map[string]bool)
	for _, ev := range events {
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && *ev.RewriteHonored {
			observed[ev.LinkedDecision] = true
		}
	}
	type key struct{ harness, session string }
	groups := make(map[key][]int)
	for i, ev := range events {
		if IsCostOnlyOutcome(ev.Outcome) || ev.Outcome == "error" || ev.Harness == "" || ev.SessionID == "" {
			continue
		}
		k := key{ev.Harness, ev.SessionID}
		groups[k] = append(groups[k], i)
	}
	for _, idxs := range groups {
		for a, idx := range idxs {
			ev := events[idx]
			if !isShift(ev) {
				continue
			}
			shifted++
			if (ev.RewriteHonored != nil && *ev.RewriteHonored) || observed[DecisionKey(ev)] {
				honored++
				continue
			}
			for _, laterIdx := range idxs[a+1:] {
				later := events[laterIdx]
				if later.FromModel == ev.ToModel && later.FromModel != "" && later.FromModel != "unknown" {
					honored++
					break
				}
			}
		}
	}
	return shifted, honored
}

// isShift reports an applied rewrite that changed the model.
func isShift(ev Event) bool {
	if ev.Outcome != OutcomeRewriteEmitted || !AppliedRewrite(ev.Outcome, ev.Corrections) {
		return false
	}
	if ev.FromModel == "" || ev.ToModel == "" || ev.ToModel == "unknown" {
		return false
	}
	return ev.FromModel != ev.ToModel
}
