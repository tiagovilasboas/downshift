// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package installer_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression: a downshift already on PATH (go install puts one in ~/go/bin)
// was left in place and the new build went to /usr/local/bin or ~/.local/bin,
// so every update after the first left two copies. Without
// DOWNSHIFT_INSTALL_DIR the installer must replace the copy the shell runs.
func TestInstallReplacesTheCopyAlreadyOnPath(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"STALE_DOWNSHIFT=1", "DOWNSHIFT_INSTALL_DIR="})
	if r.code != 0 {
		t.Fatalf("exit %d:\n%s", r.code, r.output)
	}
	existing := filepath.Join(r.home, "fakebin", "downshift")
	body, err := os.ReadFile(existing)
	if err != nil || !strings.Contains(string(body), "fake-downshift") {
		t.Fatalf("the copy on PATH was not replaced (%v): %q\n%s", err, body, r.output)
	}
	if !strings.Contains(r.output, "Updating the downshift already on PATH: "+existing) {
		t.Errorf("output must say which copy it updated:\n%s", r.output)
	}
	if strings.Contains(r.output, "Another downshift comes first on your PATH") {
		t.Errorf("an in-place update must not warn about a shadowing copy:\n%s", r.output)
	}
	for _, dir := range []string{filepath.Join(r.home, ".local", "bin"), filepath.Join(r.home, "bin")} {
		if _, err := os.Stat(filepath.Join(dir, "downshift")); err == nil {
			t.Errorf("a second copy was written to %s", dir)
		}
	}
}

// A link is not replaced: its target may belong to a package manager. The
// installer then falls back to its usual directories and keeps the warning.
func TestInstallLeavesALinkedCopyAlone(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"STALE_DOWNSHIFT_LINK=1", "DOWNSHIFT_INSTALL_DIR="})
	link := filepath.Join(r.home, "fakebin", "downshift")
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("the link on PATH was replaced (%v)\n%s", err, r.output)
	}
	if body, _ := os.ReadFile(filepath.Join(r.home, "linked-downshift")); !strings.Contains(string(body), "old") {
		t.Fatalf("the link target was overwritten: %q", body)
	}
	if strings.Contains(r.output, "Updating the downshift already on PATH") {
		t.Errorf("must not claim an in-place update:\n%s", r.output)
	}
}

// An explicit DOWNSHIFT_INSTALL_DIR still wins over the copy on PATH.
func TestInstallDirOverridesTheCopyOnPath(t *testing.T) {
	r := runInstallEnv(t, "v1.0.0", "", []string{"STALE_DOWNSHIFT=1"})
	assertInstalled(t, r)
	if body, _ := os.ReadFile(filepath.Join(r.home, "fakebin", "downshift")); !strings.Contains(string(body), "old") {
		t.Errorf("DOWNSHIFT_INSTALL_DIR was set, yet the copy on PATH was replaced: %q", body)
	}
}
