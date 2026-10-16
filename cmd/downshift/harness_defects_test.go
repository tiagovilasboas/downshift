// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Open defect (reproduction, fails on main 1c638f7): a hook failure records
// no harness, so error rows in events.jsonl cannot be attributed (the two
// INVALID_EVENT rows of 2026-10-10 have harness "").
func TestDefect_HookFailureRecordsHarness(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", path)
	var rc int
	captureStderr(func() { rc = runKiroCrewHook(strings.NewReader(`{not json`), cmdCat) })
	if rc != 0 {
		t.Fatalf("rc=%d, want fail-open 0", rc)
	}
	events, err := telemetry.ReadEventsFrom(path)
	if err != nil || len(events) != 1 {
		t.Fatalf("events=%d err=%v", len(events), err)
	}
	if events[0].Outcome != "error" || events[0].Harness != "kirocrew" {
		t.Fatalf("outcome=%q harness=%q, want error row attributed to kirocrew", events[0].Outcome, events[0].Harness)
	}
}
