// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func statusFor(t *testing.T) statusResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Host = "127.0.0.1:7474"
	rec := httptest.NewRecorder()
	newHandler("").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	return got
}

// The dashboard shows the same tokens saved as every live view and `stats`
// (telemetry.TokenSavings), work and cache apart, per harness and per agent
// type, including a child that ran on a pricier model. It sends no dollars.
func TestStatus_TokensSavedEqualTokenSavings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("HOME", t.TempDir())
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	decision := func(id, agentType string) string {
		return eventLine(ts, map[string]any{
			"harness": "claude-code", "correlation_id": id, "verdict": "DOWNSHIFT", "outcome": "rewrite_emitted",
			"requested_model": "claude-opus-5-5", "final_model": "claude-haiku-4-5", "agent_type": agentType,
		})
	}
	upshift := func(id, agentType string) string {
		return eventLine(ts, map[string]any{
			"harness": "claude-code", "correlation_id": id, "verdict": "UPSHIFT", "outcome": "rewrite_emitted",
			"requested_model": "claude-haiku-4-5", "final_model": "claude-opus-5-5", "agent_type": agentType,
		})
	}
	usage := func(id string, in, out, cached int64, baseline, actual float64) string {
		return eventLine(ts, map[string]any{
			"harness": "claude-code", "linked_decision": id, "outcome": "usage",
			"input_tokens": in, "output_tokens": out, "cached_tokens": cached,
			"baseline_cost_usd": baseline, "actual_cost_usd": actual,
		})
	}
	body := strings.Join([]string{
		decision("d1", "general-purpose"), usage("d1", 100, 300, 9000, 1.0, 0.2),
		upshift("d2", "Explore"), usage("d2", 40, 60, 0, 0.1, 0.4),
		eventLine(ts, map[string]any{"harness": "cursor", "correlation_id": "c1", "verdict": "OK", "outcome": "allow"}),
	}, "\n") + "\n"
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := telemetry.ReadEventsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	want := telemetry.TokenSavings(events)
	if want.Total.Children != 2 || want.Total.WorkSaved != 400 || want.Total.WorkPricier != 100 {
		t.Fatalf("fixture sanity: %+v", want.Total)
	}

	got := statusFor(t).Stats
	if got.TokensSaved.Tokens != want.Total || got.TokensSaved.Line != want.Total.Line() {
		t.Fatalf("dashboard tokens %+v %q != TokenSavings %+v %q", got.TokensSaved.Tokens, got.TokensSaved.Line, want.Total, want.Total.Line())
	}
	if !strings.HasPrefix(got.TokensSaved.Line, "saved 80% of work · 100% of cache") {
		t.Fatalf("line = %q", got.TokensSaved.Line)
	}
	types := telemetry.AgentTypeLines(want)
	if len(got.TokensSaved.ByAgentType) != len(types) || got.TokensSaved.ByAgentType[0].Line != types[0].Line {
		t.Fatalf("by agent type %+v, want %+v", got.TokensSaved.ByAgentType, types)
	}
	if cc, ok := got.TokensSavedByHarness["claude-code"]; !ok || cc.Tokens != want.Total {
		t.Fatalf("claude-code tokens %+v, want %+v", cc.Tokens, want.Total)
	}
	if _, ok := got.TokensSavedByHarness["cursor"]; ok {
		t.Fatal("a harness with no child token record must have no entry, never zeros")
	}
	if got.Dollars != "downshift report" {
		t.Fatalf("dollars = %q, want the report command", got.Dollars)
	}
}

// The status body carries no dollar figure at all: no key ends in _usd.
func TestStatus_SendsNoDollars(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("HOME", t.TempDir())
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	body := eventLine(ts, map[string]any{
		"harness": "claude-code", "correlation_id": "d1", "verdict": "DOWNSHIFT", "outcome": "rewrite_emitted",
		"estimated_savings": 0.8,
	}) + "\n" + eventLine(ts, map[string]any{
		"harness": "claude-code", "linked_decision": "d1", "outcome": "usage", "input_tokens": 10,
		"baseline_cost_usd": 1.0, "actual_cost_usd": 0.2,
	}) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Host = "127.0.0.1:7474"
	rec := httptest.NewRecorder()
	newHandler("").ServeHTTP(rec, req)
	raw := rec.Body.String()
	for _, key := range []string{"_usd", "est_units", "estimated_savings", "is_estimate"} {
		if strings.Contains(raw, key) {
			t.Fatalf("status body carries %q: %s", key, raw)
		}
	}
}

func TestStatus_OversizedLineIsReported(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("HOME", t.TempDir())
	huge := `{"harness":"` + strings.Repeat("x", telemetry.MaxLineBytes+1) + `"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(huge), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := statusFor(t); got.LogError == "" {
		t.Fatal("log_error must be set when a line exceeds the scanner limit")
	}
}
