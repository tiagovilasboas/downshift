// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sort"
	"time"
)

// Retry is an observed, prompt-free hint that a tier was not enough: a child
// ran on FailedTier under a Downshift rewrite, ended in error or was
// interrupted, and the same session then spawned again within the window.
//
// It is a weak signal. The next spawn may be unrelated work, so a Retry is
// never a required_tier label; it only flags where a human should look. The
// idea of anchoring on the newest event, so a recovered task is not flagged
// forever, comes from the stall detector in BerriAI/litellm (MIT); the
// implementation works on Downshift's own events, not on message history.
type Retry struct {
	Session    string
	Harness    string
	FailedTier string
	NextTier   string
	Gap        time.Duration
}

func failedEnd(s ChildStatus) bool { return s == ChildError || s == ChildInterrupted }

// RespawnAfterFailure scans events and returns one Retry per failed child
// that is followed by another decision in the same session inside window.
// Only the first following decision counts, so one failure yields at most one
// Retry. Events without a session id cannot be attributed and are skipped.
func RespawnAfterFailure(events []Event, window time.Duration) []Retry {
	type stamped struct {
		ev Event
		at time.Time
	}
	var all []stamped
	byKey := map[string]Event{}
	for _, e := range events {
		t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil || e.SessionID == "" {
			continue
		}
		all = append(all, stamped{e, t})
		if e.Outcome != OutcomeChildEnded {
			byKey[DecisionKey(e)] = e
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].at.Before(all[j].at) })

	var out []Retry
	for i, s := range all {
		if s.ev.Outcome != OutcomeChildEnded || !failedEnd(s.ev.ChildStatus) {
			continue
		}
		failed, ok := byKey[s.ev.LinkedDecision]
		if !ok {
			continue
		}
		for _, n := range all[i+1:] {
			if n.at.Sub(s.at) > window {
				break
			}
			if n.ev.SessionID != s.ev.SessionID || n.ev.Outcome == OutcomeChildEnded || n.ev.Outcome == "usage" {
				continue
			}
			out = append(out, Retry{
				Session: s.ev.SessionID, Harness: failed.Harness,
				FailedTier: failed.Tier, NextTier: n.ev.Tier, Gap: n.at.Sub(s.at),
			})
			break
		}
	}
	return out
}
