// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"strings"
	"testing"
)

func i64(n int64) *int64     { return &n }
func f64(n float64) *float64 { return &n }

func TestBuildCuration_GroupsAndLinksUsage(t *testing.T) {
	ev := []Event{
		{CorrelationID: "d1", Harness: "claude-code", Tier: "SMALL", FinalEffort: "low", Outcome: OutcomeRewriteEmitted, Verdict: "DOWNSHIFT"},
		{CorrelationID: "d2", Harness: "claude-code", Tier: "small", FinalEffort: "low", Outcome: OutcomeRewriteEmitted, Verdict: "UNKNOWN"},
		{CorrelationID: "d3", Harness: "claude-code", Tier: "mid", FinalEffort: "medium", Outcome: OutcomeAllow, Verdict: "UNKNOWN", Corrections: []string{"R9_UNCONFIDENT_UNKNOWN_MID"}},
		{CorrelationID: "d4", Harness: "codex", Tier: "frontier", FinalEffort: "high", Outcome: OutcomeRewriteEmitted, Verdict: "UPSHIFT"},
		{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "d1", AgentHash: "a1", InputTokens: i64(100), OutputTokens: i64(40), CachedTokens: i64(10), ActualCostUSD: f64(0.5)},
		{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "d1", AgentHash: "a1", InputTokens: i64(100), OutputTokens: i64(40)}, // same agent: counted once
		{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "gone", AgentHash: "a2", InputTokens: i64(7)},
		{Harness: "claude-code", Outcome: OutcomeChildEnded, ChildStatus: ChildCompleted}, // not a decision
	}
	rows := BuildCuration(ev)
	get := func(h, tier string) CurationRow {
		for _, r := range rows {
			if r.Harness == h && r.Tier == tier {
				return r
			}
		}
		t.Fatalf("no row %s/%s in %+v", h, tier, rows)
		return CurationRow{}
	}
	s := get("claude-code", "small")
	if s.Spawns != 2 || s.Applied != 2 || s.Held != 0 || s.Downshift != 1 {
		t.Errorf("small row: %+v", s)
	}
	if s.Usage != 1 || s.InputTok != 100 || s.OutputTok != 40 || s.CachedTok != 10 || s.CostUSD != 0.5 {
		t.Errorf("usage linked to d1 must count once: %+v", s)
	}
	m := get("claude-code", "mid")
	if m.Spawns != 1 || m.Held != 1 || m.Applied != 0 || m.HeldBy["R9_UNCONFIDENT_UNKNOWN_MID"] != 1 {
		t.Errorf("mid row: %+v", m)
	}
	if u := get("claude-code", "(unlinked)"); u.Usage != 1 || u.InputTok != 7 {
		t.Errorf("unlinked usage must not be dropped: %+v", u)
	}
	if c := get("codex", "frontier"); c.Upshift != 1 || c.Effort != "high" {
		t.Errorf("codex row: %+v", c)
	}
}

func TestPrintCuration_MarkdownAndEmpty(t *testing.T) {
	var b bytes.Buffer
	PrintCuration(&b, nil)
	if !strings.Contains(b.String(), "No decisions recorded") {
		t.Errorf("empty output: %q", b.String())
	}
	b.Reset()
	PrintCuration(&b, BuildCuration([]Event{{CorrelationID: "d", Harness: "cursor", Tier: "mid", FinalEffort: "medium", Outcome: OutcomeAllow, Corrections: []string{"R6"}}}))
	out := b.String()
	for _, want := range []string{"| cursor | mid | medium |", "## Held by guardrail", "cursor: R6 x1", "not a bill"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
