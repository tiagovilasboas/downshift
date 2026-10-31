// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// broadDir mirrors the closure wired into Deps.BroadDir in serveMcp: an
// explicit dir is resolved (absolute + symlinks) before the broad-root
// predicate judges it. The test exercises that contract so a relative, dotted
// or symlinked path cannot slip a broad directory past the write gate.
func broadDir(dir string) bool { return broadRoot(resolvedDir(dir)) }

func TestBroadDirResolvesBeforeJudging(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Guard against a leftover environment on platforms that read another var.
	t.Setenv("USERPROFILE", home)

	project := filepath.Join(home, "project")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	// A symlink that points at home must be seen through to home.
	linkToHome := filepath.Join(project, "up")
	if err := os.Symlink(home, linkToHome); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	broad := []string{
		"/",                              // filesystem root
		home,                             // home itself
		filepath.Dir(home),               // an ancestor of home
		filepath.Join(project, ".."),     // resolves to home
		filepath.Join(project, "..", ""), // home again, trailing element
		linkToHome,                       // symlink to home
	}
	for _, d := range broad {
		if !broadDir(d) {
			t.Errorf("broadDir(%q) = false; want true (it resolves to a broad directory)", d)
		}
	}

	safe := []string{
		project,                           // the normal project case under home
		filepath.Join(project, "sub"),     // deeper under the project
		filepath.Join(project, "a", ".."), // resolves back to the project, not home
	}
	for _, d := range safe {
		if broadDir(d) {
			t.Errorf("broadDir(%q) = true; want false (a project dir is not broad)", d)
		}
	}
}
