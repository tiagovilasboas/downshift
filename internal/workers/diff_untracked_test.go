// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package workers

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func needGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// An untracked file named like an option must be read as a path, never parsed
// by git: --output=pwned would make git write an empty file called pwned.
func TestUntrackedOptionNamedFileIsNotAnOption(t *testing.T) {
	needGit(t)
	dir, base := repoWithWork(t)
	if err := os.WriteFile(filepath.Join(dir, "--output=pwned"), []byte("x\n"), 0o644); err != nil {
		t.Skipf("file system rejects the name: %v", err)
	}
	if _, err := worktreeDiff(dir, base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "pwned")); err == nil {
		t.Fatal("git parsed the file name as --output and created pwned")
	}
}

// A symlink is skipped, not followed: its target's content never reaches the diff.
func TestUntrackedSymlinkIsSkipped(t *testing.T) {
	needGit(t)
	dir, base := repoWithWork(t)
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("TOP-SECRET-LINE\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link.txt")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	text, err := worktreeDiff(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "TOP-SECRET-LINE") {
		t.Fatalf("symlink target was followed:\n%s", text)
	}
	if !strings.Contains(text, "link.txt") || !strings.Contains(text, "skipped") {
		t.Fatalf("symlink should be reported as skipped:\n%s", text)
	}
}

func TestUntrackedRegularFileStillShowsDiff(t *testing.T) {
	needGit(t)
	dir, base := repoWithWork(t)
	text, err := worktreeDiff(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "+brand new line") {
		t.Fatalf("untracked diff missing:\n%s", text)
	}
}

func TestUntrackedPathOutsideWorktreeIsRejected(t *testing.T) {
	dir := t.TempDir()
	got := untrackedDiff(dir, filepath.Join("..", "escape.txt"))
	if !strings.Contains(got, "skipped") {
		t.Fatalf("outside path should be skipped, got %q", got)
	}
}
