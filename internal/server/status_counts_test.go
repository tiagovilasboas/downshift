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
	"time"
)

// Only an applied downshift counts as a switch with savings. Allow and held
// decisions are reported as not applied; usage records are not decisions.
func TestStatus_CountsOnlyAppliedRewrites(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	lines := []string{
		`{"timestamp":"` + ts + `","harness":"claude-code","verdict":"DOWNSHIFT","estimated_savings":0.8,"outcome":"rewrite_emitted","quota_status":"available"}`,
		`{"timestamp":"` + ts + `","harness":"claude-code","verdict":"DOWNSHIFT","estimated_savings":0.4,"outcome":"allow"}`,
		`{"timestamp":"` + ts + `","harness":"codex","verdict":"DOWNSHIFT","estimated_savings":0.5,"outcome":"rewrite_emitted","corrections":["R1_UNCONFIDENT_DOWNSHIFT"]}`,
		`{"timestamp":"` + ts + `","harness":"claude-code","verdict":"DOWNSHIFT","outcome":"usage"}`,
	}
	dir := filepath.Join(home, ".harness-downshift")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Host = "127.0.0.1:7474"
	rec := httptest.NewRecorder()
	newHandler("").ServeHTTP(rec, req)
	var got statusResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	st := got.Stats
	if st.Total != 3 || st.Down != 1 || st.NotApplied != 2 {
		t.Fatalf("total=%d down=%d not_applied=%d, want 3/1/2", st.Total, st.Down, st.NotApplied)
	}
	if st.EstUnits < 0.79 || st.EstUnits > 0.81 {
		t.Fatalf("est_units=%.2f, want 0.8 (applied downshift only)", st.EstUnits)
	}
	// Per-harness chips consume switches, while All consumes stats. The
	// API's computed flag must produce the same count in both views.
	byHarness := map[string]int{}
	for _, e := range got.Switches {
		if e.Verdict == "DOWNSHIFT" && e.Applied {
			byHarness[e.Harness]++
		}
	}
	if byHarness["claude-code"] != 1 || byHarness["codex"] != 0 {
		t.Fatalf("applied by harness = %v, want claude-code=1 codex=0", byHarness)
	}
}
