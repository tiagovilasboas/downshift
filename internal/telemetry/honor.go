// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package telemetry

// CountInferredHonored counts model changes that later look applied.
//
// A shift is rewrite_emitted with requested_model != final_model.
// It counts as honored when rewrite_honored is true on the event, or when a
// later rewrite_emitted in the same harness + hashed session arrives with
// requested_model equal to that final_model.
//
// Events without a session id cannot be inferred. The log is not rewritten.
func CountInferredHonored(events []Event) (shifted, honored int) {
	type key struct{ harness, session string }
	groups := make(map[key][]int)
	for i, ev := range events {
		if ev.Outcome != "rewrite_emitted" || ev.Harness == "" || ev.SessionID == "" {
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
			if ev.RewriteHonored != nil && *ev.RewriteHonored {
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

func isShift(ev Event) bool {
	if ev.FromModel == "" || ev.ToModel == "" || ev.ToModel == "unknown" {
		return false
	}
	return ev.FromModel != ev.ToModel
}
