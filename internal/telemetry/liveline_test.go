// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func liveDecision(sess, outcome, from, to, tier string, codes ...string) Event {
	return Event{
		CorrelationID: from + to + outcome + strings.Join(codes, ","), Harness: "claude-code", SessionID: sess,
		Outcome: outcome, Verdict: "DOWNSHIFT", FromModel: from, ToModel: to, Tier: tier, ReasonCodes: codes,
	}
}

func liveUsage(sess string, actual, baseline float64) Event {
	return Event{Outcome: OutcomeUsage, Harness: "claude-code", SessionID: sess, ActualCostUSD: f64(actual), BaselineCostUSD: f64(baseline)}
}

func TestShortModel(t *testing.T) {
	for in, want := range map[string]string{
		"claude-haiku-4-5": "haiku", "claude-haiku-4-5-20251001": "haiku", "claude-3-5-haiku-20241022": "haiku",
		"claude-opus-4-7": "opus", "gpt-5.6-sol": "gpt-5.6-sol", "claude-4": "claude-4", "": "",
	} {
		if got := ShortModel(in); got != want {
			t.Errorf("ShortModel(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSessionSummaryLine(t *testing.T) {
	const s = "sess-a"
	haiku, opus := "claude-haiku-4-5", "claude-opus-4-7"
	rewrite := func() Event { return liveDecision(s, OutcomeRewriteEmitted, opus, haiku, "small") }
	cases := []struct {
		name   string
		events []Event
		want   string
	}{
		{"empty log", nil, ""},
		{"only another session", []Event{liveDecision("sess-b", OutcomeRewriteEmitted, opus, haiku, "small")}, ""},
		{"only quota-like noise", []Event{{Outcome: "error", SessionID: s}, {Outcome: OutcomeResolved, SessionID: s}}, ""},
		{"one spawn, no cost", []Event{rewrite()}, "◆ downshift · 1 subagent · 1 on haiku · 0 on doubt"},
		{"saved", []Event{rewrite(), rewrite(), liveUsage(s, 0.10, 0.50), liveUsage(s, 0.02, 0.04)},
			"◆ downshift · 2 subagents · 2 on haiku · saved $0.42 · 0 on doubt"},
		{"loss shows as over", []Event{rewrite(), liveUsage(s, 0.50, 0.10)},
			"◆ downshift · 1 subagent · 1 on haiku · over $0.40 · 0 on doubt"},
		{"loss and gain net out signed", []Event{rewrite(), liveUsage(s, 0.50, 0.10), liveUsage(s, 0.10, 0.90)},
			"◆ downshift · 1 subagent · 1 on haiku · saved $0.40 · 0 on doubt"},
		{"allow counts the requested model", []Event{liveDecision(s, OutcomeAllow, opus, opus, "frontier"), rewrite(), rewrite()},
			"◆ downshift · 3 subagents · 2 on haiku · 0 on doubt"},
		{"doubt", []Event{
			liveDecision(s, OutcomeRewriteEmitted, "claude-sonnet-5-5", opus, "frontier", ReasonClassifierUnconfident), // written on doubt
			liveDecision(s, OutcomeAllow, opus, opus, "frontier", ReasonClassifierUnconfident, ReasonRiskFloor),        // lifted: not doubt
		}, "◆ downshift · 2 subagents · 2 on opus · 1 on doubt"},
		{"doubt that was not written is not counted", []Event{
			liveDecision(s, OutcomeAllow, opus, opus, "frontier", ReasonClassifierUnconfident),
		}, "◆ downshift · 1 subagent · 1 on opus · 0 on doubt"},
		{"unknown models are not a top model", []Event{liveDecision(s, OutcomeAllow, "unknown", "unknown", "mid")},
			"◆ downshift · 1 subagent · 0 on doubt"},
		{"unpriced usage adds no cost segment", []Event{rewrite(), {Outcome: OutcomeUsage, SessionID: s, InputTokens: i64(5)}},
			"◆ downshift · 1 subagent · 1 on haiku · 0 on doubt"},
	}
	for _, c := range cases {
		if got := SessionSummary(c.events, s).Line(); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
	if got := SessionSummary([]Event{rewrite()}, "").Line(); got != "" {
		t.Errorf("an empty session hash must match nothing, got %q", got)
	}
}

func TestReadEventsTailIsBoundedAndDropsTheCutLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	f, _ := os.Create(path)
	pad := strings.Repeat("x", 200)
	const n = 20000 // about 6 MB, far over the window
	for i := 0; i < n; i++ {
		b, _ := json.Marshal(Event{CorrelationID: pad, Outcome: OutcomeAllow, SessionID: "s", Timestamp: "t"})
		f.Write(append(b, '\n'))
	}
	f.Close()
	events, err := ReadEventsTailFrom(path, 100<<10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) == 0 || len(events) > 400 {
		t.Fatalf("tail read %d events, want a few hundred at most", len(events))
	}
	for _, ev := range events {
		if ev.SessionID != "s" {
			t.Fatalf("a cut line was parsed as an event: %+v", ev)
		}
	}
	if all, _ := ReadEventsTailFrom(path, 0); len(all) != n {
		t.Fatalf("no bound should read all %d, got %d", n, len(all))
	}
	if ev, err := ReadEventsTailFrom(filepath.Join(t.TempDir(), "none"), 1<<20); err != nil || ev != nil {
		t.Fatalf("missing log = %v, %v; want nil, nil", ev, err)
	}
}

func TestChildEndLine(t *testing.T) {
	cases := []struct {
		name string
		ev   Event
		want string
	}{
		{"full", Event{FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5-20251001", InputTokens: i64(12000), OutputTokens: i64(2000),
			ActualCostUSD: f64(0.014), BaselineCostUSD: f64(0.38)}, "↳ downshift: finished on haiku · 14k tokens · $0.01 (opus would be $0.38)"},
		{"same model has no would-be", Event{FromModel: "claude-haiku-4-5", ToModel: "claude-haiku-4-5", InputTokens: i64(900),
			ActualCostUSD: f64(0.02), BaselineCostUSD: f64(0.02)}, "↳ downshift: finished on haiku · 900 tokens · $0.02"},
		{"unknown requested model", Event{FromModel: "unknown", ToModel: "claude-haiku-4-5", ActualCostUSD: f64(0.5), BaselineCostUSD: f64(1)},
			"↳ downshift: finished on haiku · $0.50 (would be $1.00)"},
		{"no baseline", Event{FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5", InputTokens: i64(1500), ActualCostUSD: f64(0.004)},
			"↳ downshift: finished on haiku · 1.5k tokens · <$0.01"},
		{"model only", Event{ToModel: "claude-sonnet-4-5"}, "↳ downshift: finished on sonnet"},
		{"tokens only", Event{ToModel: "unknown", OutputTokens: i64(2500000)}, "↳ downshift: 2.5M tokens"},
		{"nothing known", Event{ToModel: "unknown"}, ""},
	}
	for _, c := range cases {
		if got := ChildEndLine(c.ev); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestQuietAndChildLineEnabled(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUIET", "")
	if Quiet() || !ChildLineEnabled(true) || ChildLineEnabled(false) {
		t.Fatal("default: shown only when the harness can show it")
	}
	t.Setenv("DOWNSHIFT_QUIET", "1")
	if !Quiet() || ChildLineEnabled(true) {
		t.Fatal("DOWNSHIFT_QUIET=1 must silence the line")
	}
}
