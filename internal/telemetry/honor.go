// Copyright (c) 2026 Tiago Vilas Boas.
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
// cannot be inferred. A resolved record that reports another model refutes
// the shift: it is never counted honored, whatever later spawns ask for. The
// log is not rewritten.
func CountInferredHonored(events []Event) (shifted, honored int) {
	t := TallyHonored(events)
	return t.Shifted, t.Honored
}

// HonorTally splits the shifts into those seen honored, those a linked
// "resolved" record refutes (the harness ran another model), and the rest,
// which have no evidence either way (no resolved record, no later spawn).
type HonorTally struct {
	Shifted, Honored, Refuted int
}

// Unobserved is the shifts with neither confirming nor refuting evidence.
func (t HonorTally) Unobserved() int { return t.Shifted - t.Honored - t.Refuted }

// TallyHonored is CountInferredHonored with the refuted shifts counted apart,
// so a miss can be told from a rewrite nobody observed.
func TallyHonored(events []Event) HonorTally {
	var t HonorTally
	// A "resolved" record is the harness reporting the model it chose for the
	// spawn. When it matches the written model it is direct observation.
	observed := make(map[string]bool)
	refuted := make(map[string]bool)
	for _, ev := range events {
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil {
			if *ev.RewriteHonored {
				observed[ev.LinkedDecision] = true
			} else {
				refuted[ev.LinkedDecision] = true
			}
		}
	}
	type key struct{ harness, session string }
	groups := make(map[key][]int)
	for i, ev := range events {
		if IsCostOnlyOutcome(ev.Outcome) || IsFailureOutcome(ev.Outcome) || ev.Harness == "" || ev.SessionID == "" {
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
			t.Shifted++
			if refuted[DecisionKey(ev)] && !observed[DecisionKey(ev)] {
				t.Refuted++
				continue
			}
			if (ev.RewriteHonored != nil && *ev.RewriteHonored) || observed[DecisionKey(ev)] {
				t.Honored++
				continue
			}
			for _, laterIdx := range idxs[a+1:] {
				later := events[laterIdx]
				if later.FromModel == ev.ToModel && later.FromModel != "" && later.FromModel != "unknown" {
					t.Honored++
					break
				}
			}
		}
	}
	return t
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

// RefutedRewrites returns the decisions (harness + "\n" + decision key) that
// a linked "resolved" record refutes: the harness ran another model.
func RefutedRewrites(events []Event) map[string]bool {
	out := map[string]bool{}
	for _, ev := range events {
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			out[ev.Harness+"\n"+ev.LinkedDecision] = true
		}
	}
	return out
}
