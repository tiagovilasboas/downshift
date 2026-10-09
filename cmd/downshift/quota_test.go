// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
)

func TestQuotaImportNativeAndStatuslineBridge(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	reset := time.Now().Add(time.Hour).Unix()
	b, _ := json.Marshal(map[string]any{"rate_limits": map[string]any{"five_hour": map[string]any{"used_percentage": 25, "resets_at": reset}, "seven_day": map[string]any{"used_percentage": 100, "resets_at": reset}}})
	var out, errOut bytes.Buffer
	if rc := runQuota([]string{"claude-statusline"}, bytes.NewReader(b), &out, &errOut); rc != 0 {
		t.Fatalf("bridge rc%d %s", rc, errOut.String())
	}
	if out.String() != "5h 25% · 7d 100%\n" {
		t.Fatalf("unexpected status UI %s", out.String())
	}
	s, present := quota.Load("claude-code")
	if !present || s == nil || s.Evaluate("claude-code", "m", time.Now()) != quota.Exhausted {
		t.Fatal("native bridge did not persist real exhaustion")
	}
	out.Reset()
	if rc := runQuota([]string{"status", "--harness", "claude-code"}, strings.NewReader(""), &out, &errOut); rc != 0 || !strings.Contains(out.String(), `"status":"exhausted"`) {
		t.Fatalf("status %d %s", rc, out.String())
	}
	// A context-window-only payload cannot refresh this evidence's timestamp.
	previous := s.ObservedAt
	if rc := runQuota([]string{"claude-statusline"}, strings.NewReader(`{"context_window":{"used_percentage":0}}`), &out, &errOut); rc != 0 {
		t.Fatal(rc)
	}
	s, _ = quota.Load("claude-code")
	if !s.ObservedAt.Equal(previous) {
		t.Fatal("missing rate_limits renewed budget")
	}
}

func TestQuotaImportRejectsWrongHarnessAndNormalizesProvenance(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	now := time.Now().UTC()
	used := 10.0
	s := quota.Snapshot{Version: 1, Harness: "cursor", Source: "codex-transcript", ObservedAt: now, ExpiresAt: now.Add(time.Minute), Windows: []quota.Window{{ID: "pool", Scope: "models", ModelIDs: []string{"runtime-model"}, UsedPercent: &used, ResetsAt: now.Add(time.Hour)}}}
	b, _ := json.Marshal(s)
	var out, errOut bytes.Buffer
	if rc := runQuota([]string{"import", "--harness", "codex", "--source", "normalized"}, bytes.NewReader(b), &out, &errOut); rc == 0 {
		t.Fatal("accepted wrong harness")
	}
	if rc := runQuota([]string{"import", "--harness", "cursor", "--source", "normalized"}, bytes.NewReader(b), &out, &errOut); rc != 0 {
		t.Fatalf("valid import rejected %s", errOut.String())
	}
	got, _ := quota.Load("cursor")
	if got.Source != "operator-normalized" {
		t.Fatal("operator JSON impersonated native collector")
	}
	s.ObservedAt = now.Add(time.Hour)
	s.ExpiresAt = s.ObservedAt.Add(time.Minute)
	b, _ = json.Marshal(s)
	if rc := runQuota([]string{"import", "--harness", "cursor", "--source", "normalized"}, bytes.NewReader(b), &out, &errOut); rc == 0 {
		t.Fatal("accepted future observation")
	}
}
