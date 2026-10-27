// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"math"
	"strings"
	"testing"
)

// decisionEv and usageEv build the two records one child leaves.
func decisionEv(id, outcome, agentType string, corrections ...string) Event {
	return Event{CorrelationID: id, Harness: "claude-code", Outcome: outcome, Verdict: "DOWNSHIFT",
		FromModel: "claude-opus-5-5", ToModel: "claude-haiku-4-5", AgentType: agentType, Corrections: corrections}
}

func usageEv(linked string, in, out, read, write int64, actual, baseline float64) Event {
	return Event{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: linked,
		InputTokens: i64(in), OutputTokens: i64(out), CachedTokens: i64(read + write), CacheWriteTokens: i64(write)}
}

func TestTokenSavings_PerChildTypeAndSession(t *testing.T) {
	honoredFalse := false
	events := []Event{
		// saved: applied rewrite, cheaper
		decisionEv("a1", OutcomeRewriteEmitted, "general-purpose"),
		usageEv("a1", 100, 300, 9000, 600, 0.01, 0.04),
		// resumed child: a second record of the same child folds in
		usageEv("a1", 50, 50, 1000, 0, 0.002, 0.008),
		// control: same models, never saved
		{CorrelationID: "c1", Harness: "claude-code", Outcome: OutcomeBaseline, Verdict: "DOWNSHIFT", FromModel: "claude-opus-5-5",
			ToModel: "claude-opus-5-5", AgentType: "general-purpose", ReasonCodes: []string{ReasonControl}},
		usageEv("c1", 200, 200, 4000, 0, 0.05, 0.05),
		// held by a guardrail: allow outcome with a correction
		decisionEv("h1", OutcomeAllow, "Explore", "R1"),
		usageEv("h1", 10, 90, 900, 0, 0.02, 0.02),
		// pricier: an applied upshift
		{CorrelationID: "u1", Harness: "claude-code", Outcome: OutcomeRewriteEmitted, Verdict: "UPSHIFT",
			FromModel: "claude-haiku-4-5", ToModel: "claude-opus-5-5", AgentType: "custom"},
		usageEv("u1", 40, 60, 0, 0, 0.03, 0.01),
		// refuted: the harness ran the requested model
		decisionEv("r1", OutcomeRewriteEmitted, "custom"),
		{Harness: "claude-code", Outcome: OutcomeResolved, LinkedDecision: "r1", RewriteHonored: &honoredFalse},
		usageEv("r1", 5, 5, 0, 0, 0.01, 0.04),
		// unlinked usage, no decision in the window
		usageEv("", 1, 1, 0, 0, 0.001, 0.001),
		// a usage record with no token counts is not a token observation
		{Harness: "claude-code", Outcome: OutcomeUsage, LinkedDecision: "a1"},
	}
	v := TokenSavings(events)

	if got := v.Child(events[0]); got.Children != 1 || got.SavedChildren != 1 || got.WorkTokens != 500 || got.WorkSaved != 500 ||
		got.CacheTokens != 10600 || got.CacheSaved != 10600 || got.Direction() != TokenCheaper {
		t.Errorf("saved child = %+v", got)
	}
	for _, tc := range []struct {
		id   string
		dir  string
		work int64
	}{{"c1", TokenUnchanged, 400}, {"h1", TokenUnchanged, 100}, {"u1", TokenPricier, 100}, {"r1", TokenUnchanged, 10}} {
		got := v.Child(Event{Harness: "claude-code", CorrelationID: tc.id})
		if got.Direction() != tc.dir || got.WorkTokens != tc.work || got.WorkSaved != 0 || got.CacheSaved != 0 {
			t.Errorf("%s = %+v (direction %s), want %s with %d work tokens and nothing saved", tc.id, got, got.Direction(), tc.dir, tc.work)
		}
	}

	tot := v.Total
	if tot.Children != 6 || tot.SavedChildren != 1 {
		t.Errorf("total children %d saved %d, want 6 and 1", tot.Children, tot.SavedChildren)
	}
	wantWork, wantCache := int64(500+400+100+100+10+2), int64(10600+4000+900)
	if tot.WorkTokens != wantWork || tot.CacheTokens != wantCache || tot.WorkSaved != 500 || tot.CacheSaved != 10600 || tot.WorkPricier != 100 {
		t.Errorf("total = %+v", tot)
	}
	if w := tot.WorkPercent(); math.Abs(w-100*500/float64(wantWork)) > 1e-9 {
		t.Errorf("work percent %v", w)
	}
	if c := tot.CachePercent(); math.Abs(c-100*10600/float64(wantCache)) > 1e-9 {
		t.Errorf("cache percent %v", c)
	}

	types := v.ByAgentType
	if g := types["general-purpose"]; g.Children != 2 || g.SavedChildren != 1 || g.WorkSaved != 500 {
		t.Errorf("general-purpose = %+v", g)
	}
	if e := types["Explore"]; e.Children != 1 || e.WorkPercent() != 0 {
		t.Errorf("Explore = %+v", e)
	}
	if c := types["custom"]; c.Children != 2 || c.WorkPricier != 100 || c.WorkSaved != 0 {
		t.Errorf("custom = %+v", c)
	}
	if u := types[UntypedAgent]; u.Children != 1 {
		t.Errorf("untyped (unlinked) = %+v", u)
	}
}

func TestTokenSaving_Line(t *testing.T) {
	if got := (TokenSaving{}).Line(); got != "" {
		t.Errorf("unobserved line %q, want empty", got)
	}
	s := TokenSaving{Children: 2, WorkTokens: 1000, WorkSaved: 620, CacheTokens: 100_000, CacheSaved: 58_000}
	if got, want := s.Line(), "saved 62% of work · 58% of cache"; got != want {
		t.Errorf("line %q, want %q", got, want)
	}
	s.WorkPricier = 2000
	if got := s.Line(); got != "saved 62% of work · 58% of cache · 2.0k on a pricier model" {
		t.Errorf("pricier line %q", got)
	}
}

// A zero says why: children that already ran on the cheapest model had nothing
// cheaper to move to, which is not the same as nothing being tried.
func TestTokenSaving_LineExplainsAZero(t *testing.T) {
	s := TokenSaving{Children: 2, WorkTokens: 1000, CacheTokens: 5000, AtFloor: 2}
	if got, want := s.Line(), "already on the cheapest model"; got != want {
		t.Errorf("all at floor: %q, want %q", got, want)
	}
	s.AtFloor = 1
	if got, want := s.Line(), "nothing saved yet"; got != want {
		t.Errorf("some above floor: %q, want %q", got, want)
	}
	s.WorkSaved = 400
	if got, want := s.Line(), "saved 40% of work · 0% of cache"; got != want {
		t.Errorf("a saving wins: %q, want %q", got, want)
	}
}

func TestTokensSavedEstimate(t *testing.T) {
	cases := []struct {
		name      string
		routing   TokenSaving
		compactor int64
		want      int64
		line      string
	}{
		{"zero", TokenSaving{}, 0, 0, "Tokens saved (estimate): 0"},
		{"routing only", TokenSaving{WorkSaved: 1000, CacheSaved: 500}, 0, 1500, "Tokens saved (estimate): 1.5k"},
		{"compactor only", TokenSaving{}, 2500, 2500, "Tokens saved (estimate): 2.5k"},
		{"both", TokenSaving{WorkSaved: 10000, CacheSaved: 2000}, 3000, 15000, "Tokens saved (estimate): 15k"},
		{"negative compactor ignored", TokenSaving{WorkSaved: 7}, -5, 7, "Tokens saved (estimate): 7"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := TokensSavedEstimate(c.routing, c.compactor)
			if got != c.want {
				t.Fatalf("total = %d, want %d", got, c.want)
			}
			if l := TokensSavedEstimateLine(got); l != c.line {
				t.Fatalf("line = %q, want %q", l, c.line)
			}
		})
	}
}

func TestReport_SetCompactorAddsToRouting(t *testing.T) {
	r := Report{TokensSaved: TokenSaving{WorkSaved: 100, CacheSaved: 50}}
	r.SetCompactor(4000, 1000)
	if r.TokensSavedEstimate != 1150 || r.CompactorSavedBytes != 4000 || r.CompactorTokensEstimate != 1000 {
		t.Fatalf("report = %+v", r)
	}
	out, err := ReportJSON(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"tokens_saved_estimate": 1150`, `"compactor_saved_bytes": 4000`, `"compactor_tokens_estimate": 1000`, `"tokens_saved"`} {
		if !strings.Contains(out, key) {
			t.Errorf("JSON misses %s:\n%s", key, out)
		}
	}
}
