// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
)

// TestRoutingTokens_RewrittenIsAPartOfBelow: a spawn that ran below the session
// model counts as rewritten only when the decision was an applied rewrite.
func TestRoutingTokens_RewrittenIsAPartOfBelow(t *testing.T) {
	const opus, sonnet = "claude-opus-4-8", "claude-sonnet-4-6"
	cases := []struct {
		name    string
		outcome string
		req     string // requested model (decision from_model)
		ran     string
		want    RoutingRow
	}{
		{"applied rewrite below session", OutcomeRewriteEmitted, opus, sonnet,
			RoutingRow{Spawns: 1, BelowSpawns: 1, Billable: 1700, BelowBillable: 1700, RewrittenSpawns: 1, RewrittenBillable: 1700}},
		{"kept spawn that the caller already asked below", OutcomeAllow, sonnet, sonnet,
			RoutingRow{Spawns: 1, BelowSpawns: 1, Billable: 1700, BelowBillable: 1700}},
		{"rewrite that did not go below session", OutcomeRewriteEmitted, sonnet, opus,
			RoutingRow{Spawns: 1, Billable: 1700}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := routingSpawn("claude-code", "d", opus, tc.req, tc.ran, 0)
			ev[0].Outcome = tc.outcome
			got := RoutingTokens(ev).ByHarness["claude-code"]
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestRoutingRows_HonestLabelAndRewriteLine(t *testing.T) {
	r := RoutingRow{Spawns: 3, BelowSpawns: 2, Billable: 3000, BelowBillable: 2000, RewrittenSpawns: 1, RewrittenBillable: 900}
	got := strings.Join(r.Rows(), "\n")
	for _, want := range []string{
		"ran below the session model  2.0k of 3.0k billable (67%; 2 of 3 spawns)",
		"of which by a Downshift rewrite  1 spawn (900 tokens)",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "cheaper tier") {
		t.Fatalf("old label survives:\n%s", got)
	}
}
