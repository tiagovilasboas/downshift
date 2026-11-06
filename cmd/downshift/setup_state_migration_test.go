// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/setup"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

func TestReportStateMigrationMovesAndReports(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv(paths.EnvStateDir, "")
	legacy := filepath.Join(home, paths.LegacyName)
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "events.jsonl"), []byte("e\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errw bytes.Buffer
	reportStateMigration(setup.Options{DryRun: true, Out: &out, Err: &errw})
	if !strings.Contains(out.String(), "would move") {
		t.Fatalf("dry run output: %q", out.String())
	}
	out.Reset()
	reportStateMigration(setup.Options{Out: &out, Err: &errw})
	if !strings.Contains(out.String(), "moved 1") {
		t.Fatalf("output: %q", out.String())
	}
	if _, err := os.Stat(filepath.Join(home, paths.DirName, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
}
