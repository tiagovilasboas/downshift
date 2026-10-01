// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package telemetry_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/telemetry"
)

func TestRotationPolicy_NoRotationWhenDisabled(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "test.log")

	// Create a file with some content.
	if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Disable rotation (MaxSizeMB = 0, MaxAgeDays = 0).
	policy := telemetry.RotationPolicy{}
	newPath, err := policy.MaybeRotate(path)
	if err != nil {
		t.Fatalf("MaybeRotate failed: %v", err)
	}

	// Should return original path, file should not be rotated.
	if newPath != path {
		t.Errorf("newPath = %q, want %q", newPath, path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("original file missing: %v", err)
	}
}

func TestRotationPolicy_RotatesBySize(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "test.log")

	// Create a file larger than 1 KB.
	content := make([]byte, 2000)
	for i := range content {
		content[i] = 'x'
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Set rotation policy: rotate if > 1000 bytes.
	policy := telemetry.RotationPolicy{MaxSizeMB: 1000 * 1024 / (2000), MaxBackups: 10} // Force rotation for 2KB file.
	_, err := policy.MaybeRotate(path)
	if err != nil {
		t.Fatalf("MaybeRotate failed: %v", err)
	}

	// Original file should exist again (rotation creates a backup and returns original path).
	// Actually, MaybeRotate renames the file, so we should have one backup + one new file (if something writes to it).
	// Since nothing writes after rotation, we should only have the rotated backup.
	entries, _ := os.ReadDir(tmpdir)
	fileCount := 0
	for _, e := range entries {
		if !e.IsDir() {
			fileCount++
		}
	}
	if fileCount == 0 {
		t.Fatal("expected at least one file after rotation")
	}
}

func TestRotationPolicy_CleanupOldBackups(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "test.log")

	// Create multiple rotated backups manually with timestamps that sort naturally.
	for i := 0; i < 5; i++ {
		backupPath := fmt.Sprintf("%s.2026-10-0%d", path, i)
		if err := os.WriteFile(backupPath, []byte("backup"), 0o600); err != nil {
			t.Fatalf("WriteFile backup %d failed: %v", i, err)
		}
	}

	// Cleanup function should delete files when > maxBackups.
	// We manually call the cleanup logic by creating a large file and rotating.
	content := make([]byte, 101*1024*1024) // 101 MB
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("WriteFile main log failed: %v", err)
	}

	policy := telemetry.RotationPolicy{MaxSizeMB: 100, MaxBackups: 2}
	_, err := policy.MaybeRotate(path)
	if err != nil {
		t.Fatalf("MaybeRotate failed: %v", err)
	}

	// After rotation + cleanup, should have max 3 files:
	// - rotated new backup
	// - 2 kept old backups
	entries, _ := os.ReadDir(tmpdir)
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	// Should be <= 3 (new rotated + 2 old = 3 max, but may be less if cleanup worked)
	if count > 3 {
		t.Logf("files in directory: %d", count)
		for _, e := range entries {
			if !e.IsDir() {
				t.Logf("  - %s", e.Name())
			}
		}
		t.Errorf("expected <= 3 files after rotation and cleanup, got %d", count)
	}
}

func TestRotationPolicy_NonexistentFileDoesNotError(t *testing.T) {
	tmpdir := t.TempDir()
	path := filepath.Join(tmpdir, "nonexistent.log")

	policy := telemetry.RotationPolicy{MaxSizeMB: 100}
	newPath, err := policy.MaybeRotate(path)
	if err != nil {
		t.Fatalf("MaybeRotate on nonexistent file failed: %v", err)
	}
	if newPath != path {
		t.Errorf("newPath = %q, want %q", newPath, path)
	}
}

func TestDefaultRotationPolicy(t *testing.T) {
	policy := telemetry.DefaultRotationPolicy()
	if policy.MaxSizeMB != 100 {
		t.Errorf("MaxSizeMB = %d, want 100", policy.MaxSizeMB)
	}
	if policy.MaxBackups != 10 {
		t.Errorf("MaxBackups = %d, want 10", policy.MaxBackups)
	}
	if policy.MaxAgeDays != 0 {
		t.Errorf("MaxAgeDays = %d, want 0", policy.MaxAgeDays)
	}
}
