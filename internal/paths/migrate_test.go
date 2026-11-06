// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/testenv"
)

func migHome(t *testing.T) (legacy, target string) {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv(EnvStateDir, "")
	legacy = filepath.Join(home, LegacyName)
	target = filepath.Join(home, DirName)
	if err := os.MkdirAll(filepath.Join(legacy, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"events.jsonl": "e\n", filepath.Join("sub", "x"): "x"}
	for p, c := range files {
		if err := os.WriteFile(filepath.Join(legacy, p), []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return legacy, target
}

func TestMigrateLegacyAbsentTarget(t *testing.T) {
	legacy, target := migHome(t)
	m, err := MigrateLegacy(false)
	if err != nil || len(m.Moved) != 2 {
		t.Fatalf("m=%+v err=%v", m, err)
	}
	if b, _ := os.ReadFile(filepath.Join(target, "sub", "x")); string(b) != "x" {
		t.Fatal("nested file not moved")
	}
	if _, err := os.Stat(filepath.Join(legacy, "events.jsonl")); err == nil {
		t.Fatal("legacy file still present")
	}
	if _, err := os.Stat(filepath.Join(legacy, MovedNote)); err != nil {
		t.Fatal("note missing")
	}
	if !strings.Contains(m.Describe(), "moved 2") {
		t.Fatal(m.Describe())
	}
	again, _ := MigrateLegacy(false)
	if again.Describe() != "" {
		t.Fatalf("second run not idle: %+v", again)
	}
}

func TestMigrateLegacyEmptyTarget(t *testing.T) {
	_, target := migHome(t)
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	m, err := MigrateLegacy(false)
	if err != nil || len(m.Moved) != 2 {
		t.Fatalf("m=%+v err=%v", m, err)
	}
	if _, err := os.Stat(filepath.Join(target, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyNonEmptyTargetKept(t *testing.T) {
	legacy, target := migHome(t)
	_ = os.MkdirAll(target, 0o755)
	_ = os.WriteFile(filepath.Join(target, "keep"), []byte("k"), 0o600)
	m, err := MigrateLegacy(false)
	if err != nil || !m.Both || len(m.Moved) != 0 {
		t.Fatalf("m=%+v err=%v", m, err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "events.jsonl")); err != nil {
		t.Fatal("legacy touched")
	}
	if !strings.Contains(m.Describe(), "both") {
		t.Fatal(m.Describe())
	}
}

func TestMigrateLegacyDryRun(t *testing.T) {
	legacy, target := migHome(t)
	m, err := MigrateLegacy(true)
	if err != nil || len(m.Moved) != 2 || !strings.Contains(m.Describe(), "would move") {
		t.Fatalf("m=%+v err=%v", m, err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatal("dry run created target")
	}
	if _, err := os.Stat(filepath.Join(legacy, "events.jsonl")); err != nil {
		t.Fatal("dry run moved file")
	}
}

func TestMigrateLegacyEnvOverrideSkips(t *testing.T) {
	legacy, _ := migHome(t)
	t.Setenv(EnvStateDir, t.TempDir())
	m, err := MigrateLegacy(false)
	if err != nil || m.Describe() != "" {
		t.Fatalf("m=%+v err=%v", m, err)
	}
	if _, err := os.Stat(filepath.Join(legacy, "events.jsonl")); err != nil {
		t.Fatal("env override still migrated")
	}
}

func TestCopyTreeFallback(t *testing.T) {
	src := t.TempDir()
	_ = os.MkdirAll(filepath.Join(src, "a"), 0o700)
	_ = os.WriteFile(filepath.Join(src, "a", "f"), []byte("hi"), 0o600)
	dst := filepath.Join(t.TempDir(), "out")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "a", "f")); string(b) != "hi" {
		t.Fatal("copy mismatch")
	}
}
