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

// The dashboard and `downshift stats` must report the same money for the
// same events, including a rewrite that cost more than its baseline.
func TestStatus_SavingsEqualStatsSummary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("HOME", t.TempDir())
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	usage := func(id string, baseline, actual float64) string {
		return eventLine(ts, map[string]any{
			"harness": "claude-code", "correlation_id": id, "outcome": "usage",
			"baseline_cost_usd": baseline, "actual_cost_usd": actual,
		})
	}
	body := strings.Join([]string{usage("u1", 1.0, 0.2), usage("u2", 0.1, 0.4), usage("u3", 0.5, 0.25)}, "\n") + "\n"
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	events, err := telemetry.ReadEventsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	want := telemetry.Summarize(events)
	if want.CostEvents != 3 || want.NetSavedUSD < 0.74 || want.NetSavedUSD > 0.76 {
		t.Fatalf("fixture sanity: %+v, want net 0.75", want)
	}

	got := statusFor(t)
	if got.Stats.Savings != want {
		t.Fatalf("server savings %+v != stats summary %+v", got.Stats.Savings, want)
	}
	if got.Stats.RealSavedUSD != want.NetSavedUSD || got.Stats.RealCostEvents != want.CostEvents {
		t.Fatalf("legacy keys must carry the net value: %+v", got.Stats)
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
