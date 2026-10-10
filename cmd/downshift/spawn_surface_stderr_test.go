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

	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func TestSpawnStderrSurfaceLine(t *testing.T) {
	now := time.Now().UTC()
	usedAvailable := 20.0
	usedExhausted := 100.0
	cases := []struct {
		name      string
		quota     *quota.Snapshot
		wantQuota string
	}{
		{name: "unknown", wantQuota: "unknown"},
		{name: "available", wantQuota: "available", quota: cursorQuota(now, &usedAvailable)},
		{name: "held", wantQuota: "held", quota: cursorQuota(now, &usedExhausted)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stderr, events := cursorSpawn(t, tc.quota)
			want := "downshift: honor=unobserved quota=" + tc.wantQuota + " usage=unobserved\n"
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr missing %q:\n%s", want, stderr)
			}
			if len(events) != 1 {
				t.Fatalf("events = %d, want 1", len(events))
			}
			if got := telemetry.ProjectQuota(events[0].QuotaStatus, events[0].CreditHeld); got != tc.wantQuota {
				t.Fatalf("logged quota projects to %q, want %q (status %q creditHeld %v)", got, tc.wantQuota, events[0].QuotaStatus, events[0].CreditHeld)
			}
		})
	}
}

func cursorQuota(now time.Time, used *float64) *quota.Snapshot {
	return &quota.Snapshot{
		Version: 1, Harness: "cursor", Source: "cursor-usage",
		ObservedAt: now.Add(-time.Second), ExpiresAt: now.Add(10 * time.Minute),
		Windows: []quota.Window{{
			ID: "session", Scope: "harness", UsedPercent: used, ResetsAt: now.Add(time.Hour),
		}},
	}
}

func cursorSpawn(t *testing.T, usage *quota.Snapshot) (string, []telemetry.Event) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "absent-quota.json"))
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)

	smallID := cmdCat.ModelFor("cursor", core.TierSmall).ID
	midID := cmdCat.ModelFor("cursor", core.TierMid).ID
	frontierID := cmdCat.ModelFor("cursor", core.TierFrontier).ID
	setSessionAllowlist(t, "cursor", []string{smallID, midID, frontierID})

	payload := map[string]any{
		"hook_event_name": "preToolUse",
		"tool_name":       "Task",
		"model_id":        frontierID,
		"tool_input":      map[string]any{"task": "rename the userId variable to userIdentifier", "model": frontierID},
	}
	if usage != nil {
		payload["usage_quota"] = usage
	}
	stdin, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	var stderr string
	var rc int
	captureStdout(func() {
		stderr = captureStderr(func() {
			rc = runHookAdapter(
				bytes.NewReader(stdin),
				func(b []byte) (cursor.Event, error) {
					var e cursor.Event
					return e, json.Unmarshal(b, &e)
				},
				func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, cmdCat) },
				printCursorAllow,
			)
		})
	})
	if rc != 0 {
		t.Fatalf("rc = %d, stderr = %s", rc, stderr)
	}
	events, err := telemetry.ReadEventsFrom(eventLog)
	if err != nil {
		t.Fatal(err)
	}
	return stderr, events
}
