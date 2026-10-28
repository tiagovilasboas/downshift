// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package worktree

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// layout creates base/root/inside, base/outside and base/rootbis, the last one
// a string-prefix sibling of root. It returns base and root.
func layout(t *testing.T) (string, string) {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "root")
	for _, dir := range []string{
		filepath.Join(root, "inside"),
		filepath.Join(base, "outside"),
		filepath.Join(base, "rootbis"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return base, root
}

func TestContained(t *testing.T) {
	base, root := layout(t)
	dotDot := root + string(filepath.Separator) + ".." + string(filepath.Separator) + "outside"

	tests := []struct {
		name      string
		root      string
		candidate string
		want      bool
		wantErr   bool
	}{
		{name: "root itself", root: root, candidate: root, want: true},
		{name: "existing child", root: root, candidate: filepath.Join(root, "inside"), want: true},
		{name: "missing nested child", root: root, candidate: filepath.Join(root, "new", "deep", "f.txt"), want: true},
		{name: "name starting with two dots", root: root, candidate: filepath.Join(root, "..cache"), want: true},
		{name: "string-prefix sibling", root: root, candidate: filepath.Join(base, "rootbis"), want: false},
		{name: "parent of root", root: root, candidate: base, want: false},
		{name: "unrelated existing dir", root: root, candidate: filepath.Join(base, "outside"), want: false},
		{name: "missing path outside", root: root, candidate: filepath.Join(base, "outside", "new"), want: false},
		{name: "dot-dot element refused", root: root, candidate: dotDot, wantErr: true},
		{name: "relative candidate refused", root: root, candidate: "inside", wantErr: true},
		{name: "empty candidate refused", root: root, candidate: "", wantErr: true},
		{name: "relative root refused", root: "root", candidate: root, wantErr: true},
		{name: "missing root refused", root: filepath.Join(base, "absent"), candidate: filepath.Join(base, "absent", "x"), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Contained(tc.root, tc.candidate)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Contained(%q, %q) = %v, want an error", tc.root, tc.candidate, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Contained(%q, %q) error = %v", tc.root, tc.candidate, err)
			}
			if got != tc.want {
				t.Errorf("Contained(%q, %q) = %v, want %v", tc.root, tc.candidate, got, tc.want)
			}
		})
	}
}

// TestContainedSpellingsOfOneDirectory covers the macOS alias /var to
// /private/var. t.TempDir returns the unresolved spelling and EvalSymlinks the
// resolved one; containment must hold between any mix of the two.
func TestContainedSpellingsOfOneDirectory(t *testing.T) {
	dir := t.TempDir()
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	pairs := []struct{ root, candidate string }{
		{dir, dir},
		{dir, filepath.Join(resolved, "child")},
		{resolved, filepath.Join(dir, "child", "new")},
	}
	for _, p := range pairs {
		got, err := Contained(p.root, p.candidate)
		if err != nil {
			t.Fatalf("Contained(%q, %q) error = %v", p.root, p.candidate, err)
		}
		if !got {
			t.Errorf("Contained(%q, %q) = false, want true", p.root, p.candidate)
		}
	}
}

// TestContainedSymlinks needs symlinks, which Windows grants only with
// Developer Mode or elevation, so it is skipped there.
func TestContainedSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks on Windows needs Developer Mode or elevation; runs on darwin and linux")
	}
	base, root := layout(t)
	outside := filepath.Join(base, "outside")
	rootlink := filepath.Join(base, "rootlink")
	links := map[string]string{
		filepath.Join(root, "escape"):   outside,
		filepath.Join(root, "alias"):    filepath.Join(root, "inside"),
		filepath.Join(root, "dangling"): filepath.Join(base, "missing"),
		rootlink:                        root,
	}
	for link, target := range links {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name      string
		root      string
		candidate string
		want      bool
		wantErr   bool
	}{
		{name: "symlink to outside dir", root: root, candidate: filepath.Join(root, "escape"), want: false},
		{name: "missing file through escaping symlink", root: root, candidate: filepath.Join(root, "escape", "secret.txt"), want: false},
		{name: "missing deep dir through escaping symlink", root: root, candidate: filepath.Join(root, "escape", "new", "deep"), want: false},
		{name: "symlink to inside dir", root: root, candidate: filepath.Join(root, "alias"), want: true},
		{name: "missing name through inside symlink", root: root, candidate: filepath.Join(root, "alias", "new"), want: true},
		{name: "symlinked root, child", root: rootlink, candidate: filepath.Join(root, "inside"), want: true},
		{name: "symlinked root, escape", root: rootlink, candidate: filepath.Join(root, "escape"), want: false},
		{name: "dangling symlink refused", root: root, candidate: filepath.Join(root, "dangling"), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Contained(tc.root, tc.candidate)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Contained(%q, %q) = %v, want an error", tc.root, tc.candidate, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Contained(%q, %q) error = %v", tc.root, tc.candidate, err)
			}
			if got != tc.want {
				t.Errorf("Contained(%q, %q) = %v, want %v", tc.root, tc.candidate, got, tc.want)
			}
		})
	}
}

// TestContainedDriveLetterCase flips the case of a Windows drive letter. Only
// Windows paths have volume names, so the test is skipped elsewhere.
func TestContainedDriveLetterCase(t *testing.T) {
	dir := t.TempDir()
	vol := filepath.VolumeName(dir)
	if vol == "" {
		t.Skip("drive letters exist only on Windows; this test runs there")
	}
	swapped := strings.ToLower(vol)
	if swapped == vol {
		swapped = strings.ToUpper(vol)
	}
	candidate := filepath.Join(swapped+dir[len(vol):], "child")
	got, err := Contained(dir, candidate)
	if err != nil {
		t.Fatalf("Contained(%q, %q) error = %v", dir, candidate, err)
	}
	if !got {
		t.Errorf("Contained(%q, %q) = false, want true", dir, candidate)
	}
}
