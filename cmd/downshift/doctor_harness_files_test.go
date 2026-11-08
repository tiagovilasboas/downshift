// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/hooks"
	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

type bridgeRow struct{ id, status, detail, fix string }

func collectBridge(t *testing.T, home string) ([]bridgeRow, bool) {
	t.Helper()
	var rows []bridgeRow
	add := func(id, status, detail, fix string) {
		rows = append(rows, bridgeRow{id, status, detail, fix})
	}
	return rows, checkGeneratedFiles(add, home, "opencode")
}

func bridgePath(home string) string {
	return filepath.Join(home, ".config", "opencode", "plugins", "downshift", "index.ts")
}

func writeBridgeFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A harness without its config directory is not detected: no row, no claim.
func TestGeneratedFilesNotDetectedWithoutConfigDir(t *testing.T) {
	rows, detected := collectBridge(t, t.TempDir())
	if detected || len(rows) != 0 {
		t.Fatalf("detected=%v rows=%v, want neither", detected, rows)
	}
}

// The file each state gets its own row and fix: missing, foreign, stale and
// current. The embedded copy is what setup writes, so it is the current one.
func TestGeneratedFilesReportEachState(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}
	want, err := fs.ReadFile(hooks.FS, hooks.OpenCodePlugin)
	if err != nil {
		t.Fatal(err)
	}

	rows, detected := collectBridge(t, home)
	if !detected || len(rows) != 1 || rows[0].status != doctorWarn || rows[0].fix != "downshift setup --yes" {
		t.Fatalf("missing bridge: detected=%v rows=%v", detected, rows)
	}

	writeBridgeFile(t, bridgePath(home), []byte("// my own plugin\n"))
	rows, _ = collectBridge(t, home)
	if len(rows) != 1 || rows[0].status != doctorWarn || rows[0].fix == "downshift setup --yes" {
		t.Fatalf("foreign bridge must not offer an overwrite: rows=%v", rows)
	}

	writeBridgeFile(t, bridgePath(home), []byte(configfile.OwnedPhrase+"\nold bridge\n"))
	rows, _ = collectBridge(t, home)
	if len(rows) != 1 || rows[0].status != doctorWarn || rows[0].fix != "downshift setup --yes" {
		t.Fatalf("stale bridge: rows=%v", rows)
	}

	writeBridgeFile(t, bridgePath(home), want)
	for _, name := range hooks.OpenCodeModules {
		body, err := fs.ReadFile(hooks.FS, name)
		if err != nil {
			t.Fatal(err)
		}
		writeBridgeFile(t, filepath.Join(filepath.Dir(bridgePath(home)), filepath.Base(name)), body)
	}
	rows, _ = collectBridge(t, home)
	if len(rows) != 1 || rows[0].status != doctorOK {
		t.Fatalf("current bridge: rows=%v", rows)
	}
}
