// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Unobserved and held routes stay on the JSON surface and out of credited savings.
func TestStatus_UnobservedDoesNotInflateSavingsOrHonor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("HOME", t.TempDir())
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	lines := []string{
		eventLine(ts, map[string]any{
			"harness": "cursor", "correlation_id": "cur-1", "verdict": "DOWNSHIFT",
			"estimated_savings": 0.8, "outcome": "rewrite_emitted", "quota_status": "unknown",
			"requested_model": "model-frontier", "final_model": "model-small", "session_id": "s1",
		}),
		eventLine(ts, map[string]any{
			"harness": "cursor", "correlation_id": "cur-later", "verdict": "OK",
			"outcome": "allow", "requested_model": "model-small", "final_model": "model-small",
			"session_id": "s1",
		}),
		eventLine(ts, map[string]any{
			"harness": "cursor", "correlation_id": "cur-emit", "verdict": "DOWNSHIFT",
			"estimated_savings": 0.25, "outcome": "rewrite_emitted", "quota_status": "unknown",
			"linked_decision": "cur-1", "rewrite_honored": true,
		}),
		eventLine(ts, map[string]any{
			"harness": "cursor", "outcome": "usage", "linked_decision": "cur-1",
			"estimated_savings": 0.9,
		}),
		eventLine(ts, map[string]any{
			"harness": "claude-code", "correlation_id": "cl-1", "verdict": "DOWNSHIFT",
			"estimated_savings": 0.4, "outcome": "rewrite_emitted", "quota_status": "available",
		}),
		eventLine(ts, map[string]any{
			"harness": "claude-code", "outcome": "resolved", "linked_decision": "cl-1",
			"rewrite_honored": true,
		}),
		eventLine(ts, map[string]any{
			"harness": "claude-code", "outcome": "usage", "linked_decision": "cl-1",
			"input_tokens": 12, "output_tokens": 3,
		}),
		eventLine(ts, map[string]any{
			"harness": "codex", "correlation_id": "cx-1", "verdict": "DOWNSHIFT",
			"estimated_savings": 0.6, "outcome": "rewrite_emitted",
			"quota_status": "available", "credit_held": true,
		}),
	}
	body := ""
	for _, line := range lines {
		body += line + "\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

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

	byID := map[string]rawEvent{}
	for _, sw := range got.Switches {
		if sw.Honor == "" || sw.Quota == "" || sw.Usage == "" {
			t.Fatalf("decision %s missing surface: %+v", sw.CorrelationID, sw)
		}
		byID[sw.CorrelationID] = sw
	}
	cur := byID["cur-1"]
	if cur.Honor != "unobserved" || cur.Quota != "unknown" || cur.Usage != "unobserved" {
		t.Fatalf("cursor surface = honor %s quota %s usage %s, want unobserved/unknown/unobserved", cur.Honor, cur.Quota, cur.Usage)
	}
	emit := byID["cur-emit"]
	if emit.Honor != "unobserved" || emit.Usage != "unobserved" {
		t.Fatalf("rewrite_emitted surface = honor %s usage %s, want unobserved/unobserved", emit.Honor, emit.Usage)
	}
	cl := byID["cl-1"]
	if cl.Honor != "observed" || cl.Quota != "available" || cl.Usage != "observed" {
		t.Fatalf("claude surface = honor %s quota %s usage %s, want observed/available/observed", cl.Honor, cl.Quota, cl.Usage)
	}
	cx := byID["cx-1"]
	if cx.Quota != "held" || cx.Honor != "unobserved" {
		t.Fatalf("held surface = honor %s quota %s, want unobserved/held", cx.Honor, cx.Quota)
	}
	if got.Stats.EstUnits < 0.39 || got.Stats.EstUnits > 0.41 {
		t.Fatalf("est_units = %v, want 0.4 (available claude downshift only)", got.Stats.EstUnits)
	}
	if math.Abs(got.Stats.EstUSD-0.004) > 1e-9 {
		t.Fatalf("est_usd = %v, want 0.004", got.Stats.EstUSD)
	}
}

func eventLine(ts string, fields map[string]any) string {
	fields["timestamp"] = ts
	raw, err := json.Marshal(fields)
	if err != nil {
		panic(err)
	}
	return string(raw)
}
