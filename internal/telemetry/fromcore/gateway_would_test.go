// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package fromcore

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func gatewayDecision(would string) core.Decision {
	return core.Decision{
		Harness: "opencode", Complexity: core.Medium, Tier: core.TierMid, Confident: true,
		Verdict: core.VerdictOK, Model: core.Model{ID: "m-70"},
		Baseline: core.Baseline{CurrentModel: core.Model{ID: "m-70"}},
		Shadow:   core.Shadow{GatewayWould: would},
	}
}

func TestFromDecision_RecordsGatewayWouldSlugAndTierOnly(t *testing.T) {
	ev := FromDecision(gatewayDecision("deepseek/deepseek-v4.1-flash"), "abcdef0123456789", "test")
	if ev.GatewayWould != "deepseek/deepseek-v4.1-flash" || ev.GatewayWouldTier != "mid" {
		t.Fatalf("gateway fields = %q %q", ev.GatewayWould, ev.GatewayWouldTier)
	}
	raw, _ := json.Marshal(ev)
	for _, key := range []string{`"gateway_would":"deepseek/deepseek-v4.1-flash"`, `"gateway_would_tier":"mid"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("event lacks %s: %s", key, raw)
		}
	}
}

func TestFromDecision_NoGatewayCandidateAddsNoKeys(t *testing.T) {
	raw, _ := json.Marshal(FromDecision(gatewayDecision(""), "abcdef0123456789", "test"))
	if strings.Contains(string(raw), "gateway_would") {
		t.Fatalf("event without a candidate carries gateway keys: %s", raw)
	}
}

func TestFromDecision_UntrustedGatewaySlugIsBounded(t *testing.T) {
	ev := FromDecision(gatewayDecision("has spaces and prompt text"), "abcdef0123456789", "test")
	if ev.GatewayWould != "unknown" {
		t.Fatalf("gateway_would = %q, want unknown", ev.GatewayWould)
	}
}

func TestStats_GatewayLine(t *testing.T) {
	mk := func(slug string) telemetry.Event {
		return FromDecision(gatewayDecision(slug), "abcdef0123456789", "test")
	}
	events := []telemetry.Event{mk("x/a"), mk("x/a"), mk(""), mk("")}
	s := telemetry.Aggregate(events)
	if s.GatewayObserved != 2 || s.GatewaySlugs["x/a"] != 2 {
		t.Fatalf("stats = %+v", s)
	}
	var out bytes.Buffer
	telemetry.PrintStats(events, telemetry.StatsOptions{Detail: true}, &out)
	want := "shadow: would route 2 of 4 spawns to x/a"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("stats lack %q:\n%s", want, out.String())
	}

	out.Reset()
	telemetry.PrintStats([]telemetry.Event{mk(""), mk("")}, telemetry.StatsOptions{Detail: true}, &out)
	if strings.Contains(out.String(), "shadow: would route") {
		t.Fatalf("gateway line printed without a candidate:\n%s", out.String())
	}
}
