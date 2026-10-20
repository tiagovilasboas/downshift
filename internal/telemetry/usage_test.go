// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
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
			want: telemetry.TokenUsage{InputTokens: 1000, OutputTokens: 500, CachedTokens: 500, CacheWriteTokens: 200},
		},
		{
			name: "flat top-level keys",
			in: map[string]any{
				"input_tokens":                float64(100),
				"output_tokens":               float64(50),
				"cache_creation_input_tokens": float64(10),
				"cache_read_input_tokens":     float64(20),
			},
			want: telemetry.TokenUsage{InputTokens: 100, OutputTokens: 50, CachedTokens: 30, CacheWriteTokens: 10},
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

// --- Aggregate: usage/baseline outcomes excluded from decision counts ---

func TestPrintStats_ShowsUsageLinked(t *testing.T) {
	usage := makeEvent("claude-code", "TRIVIAL", "DOWNSHIFT", 0.8)
	usage.Outcome = telemetry.OutcomeUsage
	usage.ActualCostUSD = float64Ptr(1.0)
	usage.BaselineCostUSD = float64Ptr(2.0)
	var buf strings.Builder
	telemetry.PrintStats([]telemetry.Event{usage}, telemetry.StatsOptions{Detail: true, Days: 0}, &buf)
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
	telemetry.PrintStats([]telemetry.Event{base}, telemetry.StatsOptions{Detail: true, Days: 0}, &buf)
	if !strings.Contains(buf.String(), "Baseline (no-route)") {
		t.Errorf("stats must show Baseline line, got:\n%s", buf.String())
	}
}

// An allow or held decision left the spawn on its requested model, so usage
// linked to it must price routed = requested and save nothing.

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
		if ev.Outcome == telemetry.OutcomeUsage && ev.LinkedDecision != "" && ev.HasTokenUsage() {
			priced++
		}
	}
	if priced != 1 {
		t.Fatalf("decision priced %d times, want 1", priced)
	}
}

// Identity from a session is independent from price discovery.
