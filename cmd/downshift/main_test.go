// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain isolates HOME before any test runs. Several tests drive the real
// hook runners, which append telemetry under the state directory derived from
// HOME; one that forgets t.Setenv would otherwise write fake decisions into the
// developer's own events.jsonl and skew `downshift stats`. Only HOME is set so
// tests that choose their own HOME or DOWNSHIFT_* variables keep working.
func TestMain(m *testing.M) {
	root, err := os.MkdirTemp("", "downshift-cmd-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain: temp dir:", err)
		os.Exit(1)
	}
	for _, k := range []string{"DOWNSHIFT_STATE_DIR", "DOWNSHIFT_EVENT_LOG"} {
		os.Unsetenv(k)
	}
	os.Setenv("HOME", root)
	code := m.Run()
	os.RemoveAll(root)
	os.Exit(code)
}
