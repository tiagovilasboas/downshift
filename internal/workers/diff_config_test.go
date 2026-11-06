// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package workers

import (
	"os"
	"path/filepath"
	"testing"
)

// A write worker can edit its worktree's git config. The diff Downshift
// computes on the host must not run a program that config names: not
// core.fsmonitor, not diff.external, not a textconv driver.
func TestWorktreeDiffRunsNoConfiguredProgram(t *testing.T) {
	needGit(t)
	dir, base := repoWithWork(t)
	marker := filepath.Join(t.TempDir(), "ran")
	script := filepath.Join(t.TempDir(), "evil.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, kv := range [][2]string{
		{"core.fsmonitor", script},
		{"diff.external", script},
		{"diff.evil.textconv", script},
	} {
		// git() drops GIT_* so a hook's GIT_DIR never points this at the real repo.
		git(t, dir, "config", "--local", kv[0], kv[1])
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitattributes"), []byte("* diff=evil\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := worktreeDiff(dir, base); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the worktree diff ran a program named in the worktree's git config")
	}
}
