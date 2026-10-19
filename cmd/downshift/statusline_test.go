// Copyright (c) 2026 Tiago Vilas Boas.
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

	"github.com/tiagovilasboas/downshift/internal/quota"
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
		{Harness: "claude-code", SessionID: h, Outcome: telemetry.OutcomeUsage, LinkedDecision: "d1", ActualCostUSD: ptr(0.10), BaselineCostUSD: ptr(0.52), InputTokens: iptr(600), OutputTokens: iptr(400), CachedTokens: iptr(9000)},
		{CorrelationID: "other", Harness: "claude-code", SessionID: telemetry.HashSessionID("another"), Outcome: telemetry.OutcomeAllow, FromModel: "claude-opus-4-7", ToModel: "claude-opus-4-7", Tier: "frontier"},
	} {
		if err := telemetry.AppendTo(log, ev); err != nil {
			t.Fatal(i, err)
		}
	}
}

func ptr(v float64) *float64 { return &v }
func iptr(v int64) *int64    { return &v }

func TestStatusline_RoutingSummaryForTheCurrentSession(t *testing.T) {
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	seedSession(t, log, "sess-ui")

	got := renderStatusline(t, statusInput("sess-ui", true))
	t.Logf("statusline: %s", strings.TrimSpace(got))
	want := "5h 25% · 7d 40% │ ◆ downshift · 2 subagents · 2 switched · 0 held · 2 on haiku · tokens saved work 100% · cache 100% · 0 on doubt\n"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	// Without a quota block only the routing summary prints.
	if got := renderStatusline(t, statusInput("sess-ui", false)); got != "◆ downshift · 2 subagents · 2 switched · 0 held · 2 on haiku · tokens saved work 100% · cache 100% · 0 on doubt\n" {
		t.Errorf("routing only: %q", got)
	}
	// Another session sees only its own spawns.
	if got := renderStatusline(t, statusInput("another", false)); got != "◆ downshift · 1 subagent · 0 switched · 0 held · 1 on opus · 0 on doubt\n" {
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
	if !strings.Contains(got, "2 subagents · 2 switched · 0 held · 2 on haiku") {
		t.Errorf("tail lost the current session: %q", got)
	}
	if raceEnabled {
		t.Skip("latency bound does not apply under -race (the tail read still returned the right session)")
	}
	if took > 250*time.Millisecond { // target is 50 ms; this bound only catches a full scan
		t.Errorf("render took %v on a 50 MB log, tail read expected", took)
	}
}

func TestStatusline_QuotaStaleMarkerWhileSnapshotHolds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "quota.json")
	t.Setenv("DOWNSHIFT_QUOTA_FILE", path)
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "")
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	seedSession(t, log, "sess-ui")
	storeAged := func(expiredAgo time.Duration) {
		_ = os.Remove(path) // Store refuses an older observation
		expires := time.Now().Add(-expiredAgo)
		used := 10.0
		if err := quota.Store(path, quota.Snapshot{Version: 1, Harness: "claude-code", Source: "claude-statusline",
			ObservedAt: expires.Add(-time.Minute), ExpiresAt: expires,
			Windows: []quota.Window{{ID: "5h", Scope: "harness", UsedPercent: &used, ResetsAt: time.Now().Add(time.Hour)}}}); err != nil {
			t.Fatal(err)
		}
	}
	storeAged(10 * time.Minute)
	if got := renderStatusline(t, statusInput("sess-ui", false)); got != "◆ downshift · 2 subagents · 2 switched · 0 held · 2 on haiku · tokens saved work 100% · cache 100% · 0 on doubt · quota stale\n" {
		t.Errorf("holding snapshot: %q", got)
	}
	// No spawns and an unusable session still flag the hold.
	if got := renderStatusline(t, statusInput("nobody", false)); got != "◆ downshift · quota stale\n" {
		t.Errorf("no spawns: %q", got)
	}
	// Past one hour the snapshot is absent: no marker.
	storeAged(2 * time.Hour)
	if got := renderStatusline(t, statusInput("sess-ui", false)); strings.Contains(got, "stale") {
		t.Errorf("abandoned snapshot must not be flagged: %q", got)
	}
	// A fresh native update replaces the snapshot and clears the marker.
	if got := renderStatusline(t, statusInput("sess-ui", true)); strings.Contains(got, "stale") {
		t.Errorf("fresh snapshot flagged stale: %q", got)
	}
}
