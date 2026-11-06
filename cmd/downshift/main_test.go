// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

// TestMain isolates HOME before any test runs. Several tests drive the real
// hook runners, which append telemetry under the state directory derived from
// HOME; one that forgets t.Setenv would otherwise write fake decisions into the
// developer's own events.jsonl and skew `downshift stats`. Only HOME is set so
// tests that choose their own HOME or DOWNSHIFT_* variables keep working.
func TestMain(m *testing.M) {
	// Tests run hooks; the background refresh must never start a real harness CLI.
	discoverRefreshSpawn = func(string) error { return nil }
	fromcore.Install(catalog.Load())
	root, err := os.MkdirTemp("", "downshift-cmd-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: temp dir:", err)
		os.Exit(1)
	}
	for _, k := range []string{"DOWNSHIFT_STATE_DIR", "DOWNSHIFT_EVENT_LOG"} {
		os.Unsetenv(k)
	}
	os.Setenv("HOME", root)
	// No real harness CLI (grok, claude, codex...) may run from a test: PATH is
	// an empty dir plus the system dirs, so discovery finds nothing and does not
	// shell out (grok models, system_profiler). A test that needs a CLI puts a
	// fake binary first on PATH itself.
	empty := filepath.Join(root, "empty-bin")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: empty bin dir:", err)
		os.Exit(1)
	}
	os.Setenv("PATH", strings.Join(append([]string{empty}, systemPathDirs()...), string(os.PathListSeparator)))
	os.Setenv("USERPROFILE", root) // Windows: os.UserHomeDir reads this
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
