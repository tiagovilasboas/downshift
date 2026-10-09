// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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

	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestStatus_CompactionByHarnessBytesOnly(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	logPath := filepath.Join(dir, "context-compactions.jsonl")
	body := strings.Join([]string{
		`{"timestamp":"2026-10-09T10:00:00Z","harness":"cursor","tool":"Bash","output_bytes":100,"reduced_bytes":40,"applied":true,"error":false}`,
		`{"timestamp":"2026-10-09T10:01:00Z","harness":"cursor","tool":"Bash","output_bytes":50,"reduced_bytes":10,"applied":true,"error":false}`,
		`{"timestamp":"2026-10-09T10:02:00Z","harness":"codex","tool":"Bash","output_bytes":80,"reduced_bytes":20,"applied":false,"error":true}`,
	}, "\n") + "\n"
	if err := os.WriteFile(logPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	raw := getStatus(t)
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(payload["agents"])), "[") {
		t.Fatalf("agents = %s, want a JSON array", payload["agents"])
	}
	var compaction struct {
		TokensBefore   int64                      `json:"tokens_before"`
		TokensAfter    int64                      `json:"tokens_after"`
		TokensReduced  int64                      `json:"tokens_reduced"`
		TokenState     string                     `json:"token_state"`
		ByHarness      map[string]json.RawMessage `json:"by_harness"`
		ByHarnessState string                     `json:"by_harness_state"`
	}
	if err := json.Unmarshal(payload["compaction"], &compaction); err != nil {
		t.Fatal(err)
	}
	if compaction.ByHarnessState != "observed" {
		t.Fatalf("by_harness_state = %q", compaction.ByHarnessState)
	}
	if compaction.TokensBefore != 0 || compaction.TokensAfter != 0 || compaction.TokensReduced != 0 || compaction.TokenState != "unavailable" {
		t.Fatalf("invented tokens: %+v", compaction)
	}
	assertHarnessBytes(t, compaction.ByHarness["cursor"], "cursor", 150, 100, 50)
	assertHarnessBytes(t, compaction.ByHarness["codex"], "codex", 80, 80, 0)
}

func TestStatus_BadCompactionLineHidesHarnesses(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	logPath := filepath.Join(dir, "context-compactions.jsonl")
	body := `{"timestamp":"2026-10-09T10:00:00Z","harness":"cursor","output_bytes":100,"reduced_bytes":40,"applied":true,"error":false}
{"timestamp":"2026-10-09T10:01:00Z","harness":"cursor","output_bytes":10,"reduced_bytes":1,"applied":true,"error":false,"tokens":12}
`
	if err := os.WriteFile(logPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	raw := getStatus(t)
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	var compaction struct {
		TokenState     string                     `json:"token_state"`
		TokensBefore   int64                      `json:"tokens_before"`
		ByHarness      map[string]json.RawMessage `json:"by_harness"`
		ByHarnessState string                     `json:"by_harness_state"`
	}
	if err := json.Unmarshal(payload["compaction"], &compaction); err != nil {
		t.Fatal(err)
	}
	if compaction.ByHarnessState != "unavailable" {
		t.Fatalf("by_harness_state = %q, want unavailable", compaction.ByHarnessState)
	}
	if len(compaction.ByHarness) != 0 {
		t.Fatalf("by_harness = %s, want no harness names", payload["compaction"])
	}
	if compaction.TokensBefore != 0 || compaction.TokenState != "unavailable" {
		t.Fatalf("bad log invented tokens: %+v", compaction)
	}
	if strings.Contains(string(payload["compaction"]), `"cursor"`) || strings.Contains(string(payload["compaction"]), `"codex"`) {
		t.Fatalf("compaction still names a harness: %s", payload["compaction"])
	}
}

func getStatus(t *testing.T) []byte {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Host = "127.0.0.1:7474"
	rec := httptest.NewRecorder()
	newHandler("").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	return rec.Body.Bytes()
}

func assertHarnessBytes(t *testing.T, raw json.RawMessage, name string, before, after, reduced int64) {
	t.Helper()
	if len(raw) == 0 {
		t.Fatalf("missing by_harness.%s", name)
	}
	var row map[string]any
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatal(err)
	}
	if row["unit"] != "bytes" {
		t.Fatalf("%s unit = %v", name, row["unit"])
	}
	if row["token_state"] != "unavailable" {
		t.Fatalf("%s token_state = %v", name, row["token_state"])
	}
	for key, value := range row {
		if strings.Contains(key, "token") && key != "token_state" {
			t.Fatalf("%s invented token field %s=%v", name, key, value)
		}
	}
	if int64(row["bytes_before"].(float64)) != before ||
		int64(row["bytes_after"].(float64)) != after ||
		int64(row["bytes_reduced"].(float64)) != reduced {
		t.Fatalf("%s bytes = before %v after %v reduced %v, want %d/%d/%d",
			name, row["bytes_before"], row["bytes_after"], row["bytes_reduced"], before, after, reduced)
	}
}
