// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package accounting

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func noteEv(h, id, outcome string, n *telemetry.RoutingNote) telemetry.Event {
	return telemetry.Event{CorrelationID: id, Harness: h, Outcome: outcome, RoutingNote: n,
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5"}
}

func TestBuildRoutingWhy(t *testing.T) {
	events := []telemetry.Event{
		noteEv("claude-code", "1", telemetry.OutcomeRewriteEmitted, &telemetry.RoutingNote{Verdict: "rule", RuleID: "fast-lane"}),
		noteEv("claude-code", "2", telemetry.OutcomeRewriteEmitted, &telemetry.RoutingNote{Verdict: "rule", RuleID: "fast-lane"}),
		noteEv("claude-code", "3", telemetry.OutcomeAllow, &telemetry.RoutingNote{Verdict: "guard_floor", Reason: "risk_floor"}),
		noteEv("claude-code", "4", telemetry.OutcomeAllow, nil),
		// free text is not an identifier: counted as not recorded, never shown
		noteEv("claude-code", "5", telemetry.OutcomeAllow, &telemetry.RoutingNote{Verdict: "Read /etc/passwd"}),
		noteEv("codex", "6", telemetry.OutcomeAllow, &telemetry.RoutingNote{Verdict: "kept"}),
		// not decisions: never counted
		{Harness: "codex", Outcome: telemetry.OutcomeUsage},
		{Outcome: telemetry.OutcomeAllow},
	}
	got := BuildRoutingWhy(events, Scope{Kind: ScopeAll})
	if got.Decisions != 6 || len(got.Harnesses) != 2 {
		t.Fatalf("got %+v", got)
	}
	cc := got.Harnesses[0]
	if cc.Harness != "claude-code" || cc.Decisions != 5 || cc.Recorded != 3 || cc.NotRecorded != 2 {
		t.Errorf("claude-code = %+v", cc)
	}
	wantReasons := []WhyCount{{"rule:fast-lane", 2}, {"guard_floor:risk_floor", 1}}
	if len(cc.TopReasons) != 2 || cc.TopReasons[0] != wantReasons[0] || cc.TopReasons[1] != wantReasons[1] {
		t.Errorf("top reasons = %+v, want %+v", cc.TopReasons, wantReasons)
	}
	if len(cc.ByVerdict) != 2 || cc.ByVerdict[0] != (WhyCount{"rule", 2}) {
		t.Errorf("by verdict = %+v", cc.ByVerdict)
	}
}

func TestBuildRoutingWhy_TopReasonsAreBounded(t *testing.T) {
	var events []telemetry.Event
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		events = append(events, noteEv("codex", id, telemetry.OutcomeAllow, &telemetry.RoutingNote{Verdict: "rule", RuleID: "r-" + id}))
	}
	got := BuildRoutingWhy(events, Scope{Kind: ScopeAll})
	if n := len(got.Harnesses[0].TopReasons); n != topReasonLimit {
		t.Errorf("top reasons = %d, want %d", n, topReasonLimit)
	}
}

func TestBuildRoutingWhy_EmptyIsNotNil(t *testing.T) {
	got := BuildRoutingWhy(nil, Scope{Kind: ScopeAll})
	if got.Decisions != 0 || got.Harnesses == nil {
		t.Errorf("got %+v", got)
	}
}

func TestTokensByHarness_UnobservedIsNotZero(t *testing.T) {
	five := 42.0
	events := []telemetry.Event{
		{CorrelationID: "a", Harness: "claude-code", Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT",
			FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5"},
		{Harness: "claude-code", Outcome: telemetry.OutcomeUsage, LinkedDecision: "a",
			InputTokens: i64(600), OutputTokens: i64(400)},
		{CorrelationID: "x", Harness: "codex", Outcome: telemetry.OutcomeAllow},
	}
	exec := []execwrap.Event{
		{Harness: "claude-code", RawBytes: 2000, EmittedBytes: 400},
		{RawBytes: 800, EmittedBytes: 0},
	}
	quota := map[string]QuotaUsed{"claude-code": {Status: QuotaNative, FiveHourPct: &five}}
	rows := TokensByHarness(events, exec, Scope{Kind: ScopeAll}, quota)
	by := map[string]HarnessTokens{}
	for _, r := range rows {
		by[r.Harness] = r
	}
	cc, cx, un := by["claude-code"], by["codex"], by["unknown"]
	if !cc.MovedObserved || cc.MovedToSmaller != 1000 || cc.CompactorAvoided != 400 || cc.Quota.Status != QuotaNative {
		t.Errorf("claude-code = %+v", cc)
	}
	if cx.MovedObserved || cx.MovedToSmaller != 0 || cx.CompactorAvoided != 0 || cx.Quota.Status != QuotaUnknown {
		t.Errorf("codex = %+v: no usage must be unobserved, quota unknown", cx)
	}
	if un.CompactorAvoided != 200 || un.MovedObserved {
		t.Errorf("unknown = %+v", un)
	}
	if len(rows) != 3 || rows[0].Harness != "claude-code" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestSafeIDDropsUnprintableAndOversizedIDs(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"claude-sonnet-5-5", "claude-sonnet-5-5"},
		{"gpt-5:high[1m]", "gpt-5:high[1m]"},
		{"bad id\x1b[31m", ""},
		{strings.Repeat("a", 129), ""},
	} {
		if got := safeID(tc.in); got != tc.want {
			t.Errorf("safeID(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildRoutingWhyIgnoresRowsWithoutHarness(t *testing.T) {
	// telemetry.IsDecisionRow rejects an empty harness, so such a row is not a decision.
	ev := noteEv("", "c1", "downshift", &telemetry.RoutingNote{Verdict: "kept"})
	got := BuildRoutingWhy([]telemetry.Event{ev}, Scope{Kind: ScopeAll})
	if len(got.Harnesses) != 0 || got.Decisions != 0 {
		t.Fatalf("harnesses %+v decisions %d", got.Harnesses, got.Decisions)
	}
}
