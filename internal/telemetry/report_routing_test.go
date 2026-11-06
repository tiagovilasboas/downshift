// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"strings"
	"testing"
)

func TestPrintRoutingSectionMarkdown(t *testing.T) {
	cases := []struct {
		name    string
		view    RoutingView
		want    []string
		wantNot []string
	}{
		{"observed harness",
			RoutingView{ByHarness: map[string]RoutingRow{"claude-code": {Spawns: 192, BelowSpawns: 150, Billable: 25_000_000, BelowBillable: 17_600_000, RewrittenSpawns: 46, RewrittenBillable: 5_300_000}}},
			[]string{"## Routing: tokens on a lighter model, per harness", "| claude-code | 25.0M | 17.6M (150 of 192 agents) | 70% | 5.3M (46 agents) |"},
			[]string{"cheaper", "below"}},
		{"unobserved harness reads unobserved, not 0",
			RoutingView{ByHarness: map[string]RoutingRow{"opencode": {UnobservedSpawns: 2}}},
			[]string{"| opencode | unobserved | - | - | - |", "2 agents unobserved"},
			[]string{"| opencode | 0"}},
		{"no rewrite shows a dash",
			RoutingView{ByHarness: map[string]RoutingRow{"codex": {Spawns: 2, BelowSpawns: 1, Billable: 2000, BelowBillable: 1000}}},
			[]string{"| codex | 2.0k | 1.0k (1 of 2 agents) | 50% | - |"}, nil},
		{"two harnesses are two rows, never summed",
			RoutingView{ByHarness: map[string]RoutingRow{
				"claude-code": {Spawns: 1, Billable: 1000},
				"codex":       {Spawns: 1, Billable: 3000}}},
			[]string{"| claude-code | 1.0k | 0 (0 of 1 agent) | 0% | - |", "| codex | 3.0k | 0 (0 of 1 agent) | 0% | - |"},
			[]string{"4.0k"}},
		{"fallback note and cache read",
			RoutingView{ByHarness: map[string]RoutingRow{"claude-code": {Spawns: 1, Billable: 1000, FallbackSpawns: 22, FallbackBillable: 5_800_000, CacheRead: 2000}}},
			[]string{"22 agents had no session model: 0 of 5.8M ran on a lighter model than the one requested", "cache read 2.0k"}, nil},
		{"empty view", RoutingView{}, []string{"No subagent tokens observed in this window."}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			PrintRoutingSectionMarkdown(&b, tc.view)
			out := b.String()
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			for _, w := range tc.wantNot {
				if strings.Contains(out, w) {
					t.Errorf("unexpected %q in:\n%s", w, out)
				}
			}
		})
	}
}
