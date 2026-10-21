// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestParseEvents_OversizedLineSurfacesError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	good := `{"harness":"h","outcome":"allow"}` + "\n"
	huge := `{"harness":"` + strings.Repeat("x", telemetry.MaxLineBytes+1) + `"}` + "\n"
	if err := os.WriteFile(path, []byte(good+huge+good), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := telemetry.ReadEventsFrom(path)
	if err == nil {
		t.Fatal("expected an error for a line above MaxLineBytes")
	}
	if len(events) != 1 {
		t.Fatalf("events before the bad line must be kept, got %d", len(events))
	}
}

func TestParseEvents_LargeLineUnderLimitIsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.jsonl")
	line := `{"harness":"` + strings.Repeat("x", 200*1024) + `","outcome":"allow"}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := telemetry.ReadEventsFrom(path)
	if err != nil || len(events) != 1 {
		t.Fatalf("200 KiB line: events=%d err=%v", len(events), err)
	}
}

func TestRotatedBackupCount(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	if n := telemetry.RotatedBackupCount(); n != 0 {
		t.Fatalf("no backups yet, got %d", n)
	}
	for _, name := range []string{"events.jsonl.2026-01-01T00-00-00", "events.jsonl.2026-02-01T00-00-00", "events.jsonl.lock", "events.jsonl"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if n := telemetry.RotatedBackupCount(); n != 2 {
		t.Fatalf("backups = %d, want 2 (lock and live log excluded)", n)
	}
}

// The summary carries the same savings in tokens: the cheaper tier's tokens
// are the gross, the pricier tier's tokens the overspend.
func TestSummarize_TokensByTier(t *testing.T) {
	events := []telemetry.Event{
		tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", "claude-opus-4-7", "claude-haiku-4-5"),
		tokUsage("d", "claude-code", "claude-opus-4-7", "claude-haiku-4-5", tokUse{in: 10, out: 20, cached: 100, write: 40}, 1, 4),
		tokDecision("e", telemetry.OutcomeRewriteEmitted, "UPSHIFT", "claude-haiku-4-5", "claude-opus-4-7"),
		tokUsage("e", "claude-code", "claude-haiku-4-5", "claude-opus-4-7", tokUse{in: 5, out: 5}, 4, 1),
	}
	got := telemetry.Summarize(events)
	if got.GrossSavedTokens != 130 || got.OverspendTokens != 10 || got.NetSavedTokens != 120 {
		t.Fatalf("tokens gross %d overspend %d net %d, want 130/10/120", got.GrossSavedTokens, got.OverspendTokens, got.NetSavedTokens)
	}
}
