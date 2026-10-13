// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func statusInput(session string, withQuota bool) []byte {
	m := map[string]any{"session_id": session}
	if withQuota {
		reset := time.Now().Add(time.Hour).Unix()
		m["rate_limits"] = map[string]any{
			"five_hour": map[string]any{"used_percentage": 25, "resets_at": reset},
			"seven_day": map[string]any{"used_percentage": 40, "resets_at": reset},
		}
	}
	b, _ := json.Marshal(m)
	return b
}

func renderStatusline(t *testing.T, in []byte) string {
	t.Helper()
	var out bytes.Buffer
	if rc := runQuota([]string{"claude-statusline"}, bytes.NewReader(in), &out, &bytes.Buffer{}); rc != 0 {
		t.Fatalf("statusline rc = %d", rc)
	}
	return out.String()
}

func seedSession(t *testing.T, log, session string) {
	t.Helper()
	h := telemetry.HashSessionID(session)
	for i, ev := range []telemetry.Event{
		{CorrelationID: "d1", Harness: "claude-code", SessionID: h, Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5", Tier: "small"},
		{CorrelationID: "d2", Harness: "claude-code", SessionID: h, Outcome: telemetry.OutcomeRewriteEmitted, Verdict: "DOWNSHIFT", FromModel: "claude-opus-4-7", ToModel: "claude-haiku-4-5", Tier: "small"},
		{Harness: "claude-code", SessionID: h, Outcome: telemetry.OutcomeUsage, LinkedDecision: "d1", ActualCostUSD: ptr(0.10), BaselineCostUSD: ptr(0.52)},
		{CorrelationID: "other", Harness: "claude-code", SessionID: telemetry.HashSessionID("another"), Outcome: telemetry.OutcomeAllow, FromModel: "claude-opus-4-7", ToModel: "claude-opus-4-7", Tier: "frontier"},
	} {
		if err := telemetry.AppendTo(log, ev); err != nil {
			t.Fatal(i, err)
		}
	}
}

func ptr(v float64) *float64 { return &v }

func TestStatusline_RoutingSummaryForTheCurrentSession(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	seedSession(t, log, "sess-ui")

	got := renderStatusline(t, statusInput("sess-ui", true))
	t.Logf("statusline: %s", strings.TrimSpace(got))
	want := "5h 25% · 7d 40% │ ◆ downshift · 2 subagents · 2 on haiku · saved $0.42 · 0 on doubt\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	// Without a quota block only the routing summary prints.
	if got := renderStatusline(t, statusInput("sess-ui", false)); got != "◆ downshift · 2 subagents · 2 on haiku · saved $0.42 · 0 on doubt\n" {
		t.Errorf("routing only: %q", got)
	}
	// Another session sees only its own spawns.
	if got := renderStatusline(t, statusInput("another", false)); got != "◆ downshift · 1 subagent · 1 on opus · 0 on doubt\n" {
		t.Errorf("session filter: %q", got)
	}
}

func TestStatusline_FailuresDegradeToWhatWorks(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "missing", "events.jsonl"))
	if got := renderStatusline(t, statusInput("sess-ui", true)); got != "5h 25% · 7d 40%\n" {
		t.Errorf("missing log must leave the quota part alone, got %q", got)
	}
	// An unreadable log (a directory) and unusable input print nothing, exit 0.
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_EVENT_LOG", dir)
	for name, in := range map[string][]byte{
		"log is a directory": statusInput("sess-ui", false), "garbage": []byte("{nope"),
		"no session": []byte(`{}`), "empty": nil, "bad session id": []byte(`{"session_id":"a b/c"}`),
	} {
		if got := renderStatusline(t, in); got != "" {
			t.Errorf("%s: printed %q", name, got)
		}
	}
}

func TestStatusline_FastOnALargeLogAndReadsOnlyTheTail(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	// 50 MB of another session's history, then this session at the end.
	f, err := os.Create(log)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(telemetry.Event{CorrelationID: strings.Repeat("o", 300), Harness: "claude-code", SessionID: telemetry.HashSessionID("old-session"), Outcome: telemetry.OutcomeAllow, FromModel: "claude-opus-4-7", ToModel: "claude-opus-4-7"})
	old = append(old, '\n')
	for written := 0; written < 50<<20; written += len(old) {
		f.Write(old)
	}
	f.Close()
	seedSession(t, log, "sess-ui")

	start := time.Now()
	got := renderStatusline(t, statusInput("sess-ui", false))
	took := time.Since(start)
	t.Logf("50 MB log rendered in %v", took)
	if !strings.Contains(got, "2 subagents · 2 on haiku") {
		t.Errorf("tail lost the current session: %q", got)
	}
	if took > 250*time.Millisecond { // target is 50 ms; this bound only catches a full scan
		t.Errorf("render took %v on a 50 MB log, tail read expected", took)
	}
}
