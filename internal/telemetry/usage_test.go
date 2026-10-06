// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/harness-downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/harness-downshift/internal/catalog"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

// --- TokenUsageFromPayload: defensive extraction ---

func TestTokenUsageFromPayload_Defensive(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]any
		want telemetry.TokenUsage
	}{
		{name: "nil map", in: nil, want: telemetry.TokenUsage{}},
		{name: "empty map", in: map[string]any{}, want: telemetry.TokenUsage{}},
		{
			name: "nested usage object",
			in: map[string]any{"usage": map[string]any{
				"input_tokens":                float64(1000),
				"output_tokens":               float64(500),
				"cache_creation_input_tokens": float64(200),
				"cache_read_input_tokens":     float64(300),
			}},
			want: telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 500, CachedTokens: 500},
		},
		{
			name: "flat top-level keys",
			in: map[string]any{
				"input_tokens":                float64(100),
				"output_tokens":               float64(50),
				"cache_creation_input_tokens": float64(10),
				"cache_read_input_tokens":     float64(20),
			},
			want: telemetry.TokenUsage{InputTokens: 100, OutputTokens: 50, CachedTokens: 30},
		},
		{
			name: "nested wins over flat",
			in: map[string]any{
				"input_tokens": float64(999),
				"usage":        map[string]any{"input_tokens": float64(100)},
			},
			want: telemetry.TokenUsage{InputTokens: 100},
		},
		{
			name: "invalid nested falls back to flat",
			in: map[string]any{
				"input_tokens": float64(50),
				"usage":        map[string]any{"input_tokens": "a lot"},
			},
			want: telemetry.TokenUsage{InputTokens: 50},
		},
		{
			name: "wrong types to zero",
			in: map[string]any{"usage": map[string]any{
				"input_tokens":                "1000",
				"output_tokens":               true,
				"cache_creation_input_tokens": nil,
				"cache_read_input_tokens":     map[string]any{"n": 1},
			}},
			want: telemetry.TokenUsage{},
		},
		{
			name: "negatives to zero",
			in: map[string]any{"usage": map[string]any{
				"input_tokens":                float64(-5),
				"output_tokens":               -10,
				"cache_creation_input_tokens": float64(-1),
				"cache_read_input_tokens":     int64(-2),
			}},
			want: telemetry.TokenUsage{},
		},
		{
			name: "fractional truncated",
			in:   map[string]any{"usage": map[string]any{"input_tokens": float64(10.9)}},
			want: telemetry.TokenUsage{InputTokens: 10},
		},
		{
			name: "non-map usage ignored, flat still read",
			in: map[string]any{
				"usage":         "garbage",
				"output_tokens": float64(7),
			},
			want: telemetry.TokenUsage{OutputTokens: 7},
		},
		{
			name: "everything else ignored",
			in: map[string]any{
				"hook_event_name": "PostToolUse",
				"model":           "m",
				"prompt":          "do not persist",
				"usage":           map[string]any{"input_tokens": float64(3)},
			},
			want: telemetry.TokenUsage{InputTokens: 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telemetry.TokenUsageFromPayload(tt.in); got != tt.want {
				t.Errorf("TokenUsageFromPayload() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// --- FillRealCost: math matches CostUSD ---

func TestFillRealCost_MatchesCostUSD(t *testing.T) {
	baseline := core.Model{ID: "pricey", InputM: 10, OutputM: 10}
	routed := core.Model{ID: "cheap", InputM: 1, OutputM: 1}
	ev := makeEvent("claude-code", "TRIVIAL", "DOWNSHIFT", 0.5)
	ev.InputTokens = int64Ptr(1_000_000)
	ev.OutputTokens = int64Ptr(500_000)
	ev.CachedTokens = int64Ptr(500_000)

	telemetry.FillRealCost(&ev, baseline, routed)
	usage := telemetry.TokenUsage{InputTokens: 1_000_000, OutputTokens: 500_000, CachedTokens: 500_000}
	if ev.ActualCostUSD == nil || ev.BaselineCostUSD == nil {
		t.Fatal("FillRealCost left cost pointers nil")
	}
	if math.Abs(*ev.ActualCostUSD-telemetry.CostUSD(routed, usage)) > 1e-9 {
		t.Errorf("ActualCostUSD = %f, want CostUSD(routed)", *ev.ActualCostUSD)
	}
	if math.Abs(*ev.BaselineCostUSD-telemetry.CostUSD(baseline, usage)) > 1e-9 {
		t.Errorf("BaselineCostUSD = %f, want CostUSD(baseline)", *ev.BaselineCostUSD)
	}
	if !ev.HasRealCost() {
		t.Error("HasRealCost = false after FillRealCost")
	}
	// Downshift math: baseline $20 + ... vs routed: positive savings.
	if got := ev.RealSavedUSD(); math.Abs(got-((*ev.BaselineCostUSD)-(*ev.ActualCostUSD))) > 1e-9 {
		t.Errorf("RealSavedUSD = %f, want baseline-actual", got)
	}
}

func TestFillRealCost_NilSafe(t *testing.T) {
	m := core.Model{ID: "m", InputM: 1, OutputM: 1}
	telemetry.FillRealCost(nil, m, m) // must not panic
}

// --- Aggregate: usage/baseline outcomes excluded from decision counts ---

func TestAggregate_UsageExcludedFromDecisionCounts(t *testing.T) {
	decisions := []telemetry.Event{
		makeEvent("claude-code", "TRIVIAL", "DOWNSHIFT", 0.8),
		makeEvent("claude-code", "COMPLEX", "OK", 0),
	}
	before := telemetry.Aggregate(decisions)

	usage := makeEvent("claude-code", "TRIVIAL", "DOWNSHIFT", 0.8)
	usage.Outcome = telemetry.OutcomeUsage
	usage.Verdict = "DOWNSHIFT" // copied verdict must not double-count
	usage.InputTokens = int64Ptr(1000)
	usage.OutputTokens = int64Ptr(500)
	usage.ActualCostUSD = float64Ptr(1.0)
	usage.BaselineCostUSD = float64Ptr(4.0)

	after := telemetry.Aggregate(append(append([]telemetry.Event{}, decisions...), usage))
	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"Total", after.Total, before.Total},
		{"Downshifted", after.Downshifted, before.Downshifted},
		{"Upshifted", after.Upshifted, before.Upshifted},
		{"OK", after.OK, before.OK},
		{"Unknown", after.Unknown, before.Unknown},
	} {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d (usage event must not move it)", c.name, c.got, c.want)
		}
	}
	if after.NormBaseline != before.NormBaseline || after.NormRouted != before.NormRouted {
		t.Errorf("normalised units moved: before (%v,%v) after (%v,%v)",
			before.NormBaseline, before.NormRouted, after.NormBaseline, after.NormRouted)
	}
	if after.ByComplexity["TRIVIAL"] != before.ByComplexity["TRIVIAL"] {
		t.Errorf("ByComplexity moved: before %v after %v", before.ByComplexity, after.ByComplexity)
	}
	if after.RealCostEvents != 1 {
		t.Errorf("RealCostEvents = %d, want 1", after.RealCostEvents)
	}
	if math.Abs(after.RealSavedUSD-3.0) > 1e-9 {
		t.Errorf("RealSavedUSD = %f, want 3.0", after.RealSavedUSD)
	}
	if after.UsageLinked != 1 {
		t.Errorf("UsageLinked = %d, want 1", after.UsageLinked)
	}

	// Reserved "baseline" outcome behaves the same, even without costs.
	base := makeEvent("claude-code", "SIMPLE", "OK", 0)
	base.Outcome = telemetry.OutcomeBaseline
	mixed := telemetry.Aggregate([]telemetry.Event{decisions[0], usage, base})
	if mixed.Total != 1 || mixed.Downshifted != 1 {
		t.Errorf("mixed Total/Downshifted = %d/%d, want 1/1", mixed.Total, mixed.Downshifted)
	}
	if mixed.UsageLinked != 1 {
		t.Errorf("mixed UsageLinked = %d, want 1 (usage only)", mixed.UsageLinked)
	}
	if mixed.Baseline != 1 {
		t.Errorf("mixed Baseline = %d, want 1", mixed.Baseline)
	}
	if mixed.RealCostEvents != 1 {
		t.Errorf("mixed RealCostEvents = %d, want 1 (costless baseline adds none)", mixed.RealCostEvents)
	}
}

func TestPrintStats_ShowsUsageLinked(t *testing.T) {
	usage := makeEvent("claude-code", "TRIVIAL", "DOWNSHIFT", 0.8)
	usage.Outcome = telemetry.OutcomeUsage
	usage.ActualCostUSD = float64Ptr(1.0)
	usage.BaselineCostUSD = float64Ptr(2.0)
	var buf strings.Builder
	telemetry.PrintStats([]telemetry.Event{usage}, telemetry.StatsOptions{Days: 0}, &buf)
	if !strings.Contains(buf.String(), "Usage linked") {
		t.Errorf("stats must show Usage linked line, got:\n%s", buf.String())
	}
}

// --- Handler end-to-end: hermetic temp log ---

func usageTestCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	t.Setenv("HOME", t.TempDir()) // isolate user catalog override
	return catalog.Load()
}

func seedDecision(t *testing.T, path, sessionHash, fromID, toID string) {
	t.Helper()
	ev := makeEvent("claude-code", "TRIVIAL", "DOWNSHIFT", 0.8)
	ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	ev.Outcome = "rewrite_emitted"
	ev.FromModel = fromID
	ev.ToModel = toID
	ev.SessionID = sessionHash
	if err := telemetry.AppendTo(path, ev); err != nil {
		t.Fatalf("seed decision: %v", err)
	}
}

func TestHandlePostToolUse_EndToEnd_Linked(t *testing.T) {
	cat := usageTestCatalog(t)
	smallID := cat.ModelFor("claude-code", core.TierSmall).ID
	frontierID := cat.ModelFor("claude-code", core.TierFrontier).ID
	if smallID == "" || frontierID == "" {
		t.Fatal("catalog missing claude-code tier models")
	}
	small, _ := cat.LookupByID("claude-code", smallID)
	frontier, _ := cat.LookupByID("claude-code", frontierID)

	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	sessionHash := telemetry.HashSessionID("sess-e2e")
	seedDecision(t, eventLog, sessionHash, frontierID, smallID)

	payload, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"tool_name":       "Task",
		"model":           smallID,
		"session_id":      "sess-e2e",
		"usage": map[string]any{
			"input_tokens":                float64(1000),
			"output_tokens":               float64(500),
			"cache_creation_input_tokens": float64(200),
			"cache_read_input_tokens":     float64(300),
		},
	})
	out, _, linked := claudecode.HandlePostToolUse(payload, "test", cat)
	if !linked {
		t.Fatal("HandlePostToolUse linked = false, want true (session match)")
	}
	if raw, _ := json.Marshal(out); string(raw) != "{}" {
		t.Errorf("PostToolUse output = %s, want {}", raw)
	}

	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil || len(events) != 2 {
		t.Fatalf("events = %d, err = %v (want decision + usage)", len(events), err)
	}
	// History untouched: the decision keeps no real cost.
	if events[0].HasRealCost() {
		t.Error("decision event was modified in place — v1 must append, never rewrite")
	}
	u := events[1]
	if u.Outcome != telemetry.OutcomeUsage {
		t.Errorf("Outcome = %q, want usage", u.Outcome)
	}
	if u.Verdict != "DOWNSHIFT" {
		t.Errorf("Verdict = %q, want copied DOWNSHIFT", u.Verdict)
	}
	if u.SessionID != sessionHash || u.FromModel != frontierID || u.ToModel != smallID {
		t.Errorf("provenance not copied: %+v", u)
	}
	wantUsage := telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 500, CachedTokens: 500}
	if u.TokenUsageOrZero() != wantUsage {
		t.Errorf("tokens = %+v, want %+v", u.TokenUsageOrZero(), wantUsage)
	}
	if u.ActualCostUSD == nil || math.Abs(*u.ActualCostUSD-telemetry.CostUSD(small, wantUsage)) > 1e-9 {
		t.Errorf("ActualCostUSD = %v, want CostUSD(routed)", u.ActualCostUSD)
	}
	if u.BaselineCostUSD == nil || math.Abs(*u.BaselineCostUSD-telemetry.CostUSD(frontier, wantUsage)) > 1e-9 {
		t.Errorf("BaselineCostUSD = %v, want CostUSD(baseline)", u.BaselineCostUSD)
	}

	s := telemetry.Aggregate(events)
	if s.Total != 1 || s.Downshifted != 1 {
		t.Errorf("Total/Downshifted = %d/%d, want 1/1", s.Total, s.Downshifted)
	}
	if s.RealCostEvents != 1 || s.UsageLinked != 1 {
		t.Errorf("RealCostEvents/UsageLinked = %d/%d, want 1/1", s.RealCostEvents, s.UsageLinked)
	}
	if math.Abs(s.NormBaseline-1.0) > 1e-9 {
		t.Errorf("NormBaseline = %f, want 1.0 (usage adds none)", s.NormBaseline)
	}
}

func TestHandlePostToolUse_EndToEnd_Unmatched(t *testing.T) {
	cat := usageTestCatalog(t)
	smallID := cat.ModelFor("claude-code", core.TierSmall).ID
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	payload, _ := json.Marshal(map[string]any{
		"hook_event_name": "PostToolUse",
		"model":           smallID,
		"session_id":      "no-such-session",
		"usage":           map[string]any{"input_tokens": float64(10), "output_tokens": float64(5)},
	})
	_, _, linked := claudecode.HandlePostToolUse(payload, "test", cat)
	if linked {
		t.Error("linked = true, want false (empty log)")
	}
	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil || len(events) != 1 {
		t.Fatalf("events = %d, err = %v, want 1 unmatched record", len(events), err)
	}
	if events[0].Verdict != "UNKNOWN" {
		t.Errorf("Verdict = %q, want UNKNOWN", events[0].Verdict)
	}
	if !events[0].HasRealCost() {
		t.Error("unmatched record should still price the routed spend")
	}
	if got := events[0].RealSavedUSD(); got != 0 {
		t.Errorf("RealSavedUSD = %f, want 0 (baseline falls back to routed)", got)
	}
}

func TestHandlePostToolUse_FailOpen(t *testing.T) {
	cat := usageTestCatalog(t)
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	for name, raw := range map[string][]byte{
		"malformed json": []byte(`{not valid`),
		"no usage":       []byte(`{"hook_event_name":"PostToolUse","model":"m"}`),
		"empty usage":    []byte(`{"hook_event_name":"PostToolUse","usage":{}}`),
	} {
		out, _, linked := claudecode.HandlePostToolUse(raw, "test", cat)
		if linked {
			t.Errorf("%s: linked = true, want false", name)
		}
		if b, _ := json.Marshal(out); string(b) != "{}" {
			t.Errorf("%s: output = %s, want {}", name, b)
		}
	}
	events, _ := telemetry.ReadEventsFrom(eventLog)
	if len(events) != 0 {
		t.Errorf("fail-open paths wrote %d events, want 0", len(events))
	}
}

func TestPrintStats_ShowsBaseline(t *testing.T) {
	base := makeEvent("claude-code", "SIMPLE", "OK", 0)
	base.Outcome = telemetry.OutcomeBaseline
	var buf strings.Builder
	telemetry.PrintStats([]telemetry.Event{base}, telemetry.StatsOptions{Days: 0}, &buf)
	if !strings.Contains(buf.String(), "Baseline (no-route)") {
		t.Errorf("stats must show Baseline line, got:\n%s", buf.String())
	}
}

// An allow or held decision left the spawn on its requested model, so usage
// linked to it must price routed = requested and save nothing.
func TestHandlePostToolUse_HeldDecisionSavesNothing(t *testing.T) {
	cat := usageTestCatalog(t)
	midID := cat.ModelFor("claude-code", core.TierMid).ID
	frontierID := cat.ModelFor("claude-code", core.TierFrontier).ID
	for name, mutate := range map[string]func(*telemetry.Event){
		"allow": func(ev *telemetry.Event) { ev.Outcome = telemetry.OutcomeAllow },
		"held": func(ev *telemetry.Event) {
			ev.Corrections = []string{core.RuleUnconfidentDowngrade}
			ev.SafeVerdict = "OK"
		},
	} {
		t.Run(name, func(t *testing.T) {
			eventLog := filepath.Join(t.TempDir(), "events.jsonl")
			t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
			sessionHash := telemetry.HashSessionID("sess-held")
			ev := makeEvent("claude-code", "MEDIUM", "DOWNSHIFT", 0.4)
			ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
			ev.CorrelationID = telemetry.NewCorrelationID()
			ev.Outcome = telemetry.OutcomeRewriteEmitted
			ev.FromModel, ev.ToModel, ev.SessionID = frontierID, midID, sessionHash
			mutate(&ev)
			if err := telemetry.AppendTo(eventLog, ev); err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(map[string]any{
				"session_id": "sess-held",
				"usage":      map[string]any{"input_tokens": float64(100000), "output_tokens": float64(20000)},
			})
			if _, _, linked := claudecode.HandlePostToolUse(payload, "test", cat); !linked {
				t.Fatal("usage should link to the session's decision")
			}
			events, _ := telemetry.ReadEventsFrom(eventLog)
			s := telemetry.Aggregate(events)
			if s.RealSavedUSD != 0 {
				t.Fatalf("RealSavedUSD = %.2f, want 0 (the spawn ran on %s)", s.RealSavedUSD, frontierID)
			}
			if u := events[len(events)-1]; u.ToModel != frontierID {
				t.Fatalf("usage ToModel = %q, want requested %q", u.ToModel, frontierID)
			}
		})
	}
}

// A decision is priced once: a second usage record for the same session
// must not link to (and re-price) the same decision.
func TestHandlePostToolUse_DecisionPricedOnce(t *testing.T) {
	cat := usageTestCatalog(t)
	smallID := cat.ModelFor("claude-code", core.TierSmall).ID
	frontierID := cat.ModelFor("claude-code", core.TierFrontier).ID
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	sessionHash := telemetry.HashSessionID("sess-once")
	seedDecision(t, eventLog, sessionHash, frontierID, smallID)

	payload, _ := json.Marshal(map[string]any{
		"session_id": "sess-once",
		"usage":      map[string]any{"input_tokens": float64(1000), "output_tokens": float64(500)},
	})
	if _, _, linked := claudecode.HandlePostToolUse(payload, "test", cat); !linked {
		t.Fatal("first usage must link")
	}
	if _, _, linked := claudecode.HandlePostToolUse(payload, "test", cat); linked {
		t.Fatal("second usage must not re-price the same decision")
	}
	events, _ := telemetry.ReadEventsFrom(eventLog)
	priced := 0
	for _, ev := range events {
		if ev.Outcome == telemetry.OutcomeUsage && ev.RealSavedUSD() > 0 {
			priced++
		}
	}
	if priced != 1 {
		t.Fatalf("decision priced %d times, want 1", priced)
	}
}
