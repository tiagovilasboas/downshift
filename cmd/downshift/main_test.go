// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// TestMain isolates HOME before any test runs. Several tests drive the real
// hook runners, which append telemetry under the state directory derived from
// HOME; one that forgets t.Setenv would otherwise write fake decisions into the
// developer's own events.jsonl and skew `downshift stats`. Only HOME is set so
// tests that choose their own HOME or DOWNSHIFT_* variables keep working.
func TestMain(m *testing.M) {
	// Tests run hooks; the background refresh must never start a real harness CLI.
	discoverRefreshSpawn = func(string) error { return nil }
	telemetry.DirectionCatalog = catalog.Load()
	root, err := os.MkdirTemp("", "downshift-cmd-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: temp dir:", err)
		os.Exit(1)
	}
	for _, k := range []string{"DOWNSHIFT_STATE_DIR", "DOWNSHIFT_EVENT_LOG"} {
		os.Unsetenv(k)
	}
	os.Setenv("HOME", root)
	// Keep the built-in control group off so hook tests see a rewrite every
	// time; control-group tests set DOWNSHIFT_CONTROL_SHARE themselves.
	os.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
