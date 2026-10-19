// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package codeintel

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRecordTelemetryToRefusesSymlinkAndKeepsTargetUntouched(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "victim.txt")
	if err := os.WriteFile(target, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "dci-events.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := RecordTelemetryTo(link, Telemetry{}); err == nil {
		t.Fatal("a symlinked log must be refused")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Fatalf("symlink target mode changed to %v", info.Mode().Perm())
	}
	if data, _ := os.ReadFile(target); string(data) != "keep\n" {
		t.Fatalf("symlink target was written: %q", data)
	}
}

func TestRecordTelemetryToCreatesPrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "dci-events.jsonl")
	if err := RecordTelemetryTo(path, Telemetry{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("new log mode = %v, want 0600", info.Mode().Perm())
	}
}
