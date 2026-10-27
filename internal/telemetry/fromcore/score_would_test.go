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

func scoreDecision() core.Decision {
	return core.Decision{
		Harness: "codex", Complexity: core.Medium, Tier: core.TierMid, Confident: true,
		Verdict: core.VerdictOK, Model: core.Model{ID: "m-70"},
		Baseline: core.Baseline{
			CurrentModel: core.Model{ID: "m-70"},
		},
		Shadow: core.Shadow{
			ScoreWould:          "m-40",
			ScoreDiffers:        true,
			ScoreWouldCostRatio: 0.123456,
		},
	}
}

func TestFromDecision_RecordsScoreWouldPromptFree(t *testing.T) {
	ev := FromDecision(scoreDecision(), "abcdef0123456789", "test")
	if ev.ScoreWould != "m-40" || !ev.ScoreDiffers || ev.ScoreWouldCostRatio != 0.1235 {
		t.Fatalf("score fields = %q %v %v", ev.ScoreWould, ev.ScoreDiffers, ev.ScoreWouldCostRatio)
	}
	raw, _ := json.Marshal(ev)
	for _, key := range []string{`"score_would":"m-40"`, `"score_differs":true`, `"score_would_cost_ratio":0.1235`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("event lacks %s: %s", key, raw)
		}
	}
	// A shadow pick is not a reason, and an event with no pick stays unchanged.
	for _, c := range ev.ReasonCodes {
		if c == telemetry.ReasonScoreRouted {
			t.Fatalf("shadow pick produced %s", c)
		}
	}
	d := scoreDecision()
	d.Shadow.ScoreWould, d.Shadow.ScoreDiffers, d.Shadow.ScoreWouldCostRatio = "", false, 0
	raw, _ = json.Marshal(FromDecision(d, "abcdef0123456789", "test"))
	if strings.Contains(string(raw), "score") {
		t.Fatalf("event without a pick carries score keys: %s", raw)
	}
}

func TestFromDecision_UntrustedScoreIDIsBounded(t *testing.T) {
	d := scoreDecision()
	d.Shadow.ScoreWould = "has spaces and prompt text"
	if got := FromDecision(d, "abcdef0123456789", "test").ScoreWould; got != "unknown" {
		t.Fatalf("score_would = %q, want unknown", got)
	}
}

func TestReasonCodes_ScoreRoutedOnlyWhenApplied(t *testing.T) {
	d := scoreDecision()
	d.Shadow.ScoreApplied = true
	found := false
	for _, c := range ReasonCodes(d) {
		found = found || c == telemetry.ReasonScoreRouted
	}
	if !found {
		t.Fatalf("applied pick has no %s: %v", telemetry.ReasonScoreRouted, ReasonCodes(d))
	}
}

func TestStats_ScoreLine(t *testing.T) {
	mk := func(would string, differs bool, ratio float64) telemetry.Event {
		ev := FromDecision(scoreDecision(), "abcdef0123456789", "test")
		ev.ScoreWould, ev.ScoreDiffers, ev.ScoreWouldCostRatio = would, differs, ratio
		return ev
	}
	events := []telemetry.Event{mk("m-40", true, 0.5), mk("m-70", false, 1), mk("m-90", true, 0), mk("", false, 0)}
	s := telemetry.Aggregate(events)
	if s.ScoreObserved != 3 || s.ScoreDiffers != 2 || s.ScoreRatioN != 2 {
		t.Fatalf("stats = %+v", s)
	}
	var out bytes.Buffer
	telemetry.PrintStats(events, telemetry.StatsOptions{Detail: true}, &out)
	want := "Score pick differs    2 of 3 decisions (67%), list price 25% lower on average"
	if !strings.Contains(out.String(), want) {
		t.Fatalf("stats lack %q:\n%s", want, out.String())
	}
	// No pick anywhere: the report has no score line.
	out.Reset()
	telemetry.PrintStats([]telemetry.Event{mk("", false, 0)}, telemetry.StatsOptions{Detail: true}, &out)
	if strings.Contains(out.String(), "Score pick") {
		t.Fatalf("score line printed without picks:\n%s", out.String())
	}
}
