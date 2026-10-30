// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func skipWithoutUnixModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not enforced on Windows")
	}
}

// An existing file keeps its mode; a new one is 0644. A private agent file does
// not become world-readable by being updated.
func TestPolicyApply_KeepsFileMode(t *testing.T) {
	skipWithoutUnixModes(t)
	cases := []struct {
		name     string
		existing bool
		mode     os.FileMode
	}{
		{"private file stays 0600", true, 0o600},
		{"group-readable file stays 0640", true, 0o640},
		{"public file stays 0644", true, 0o644},
		{"executable bit is kept", true, 0o755},
		{"new file is 0644", false, 0o644},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("scout", "small", "Scouts")))
			path := filepath.Join(dir, ".claude", "agents", "scout.md")
			if c.existing {
				writeTestFile(t, path, "---\nname: scout\ndescription: d\nmodel: old\n---\nbody\n", c.mode)
			}
			if code, _, stderr := runApply(t, dir, home); code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			fi, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if fi.Mode().Perm() != c.mode {
				t.Errorf("mode = %v, want %v", fi.Mode().Perm(), c.mode)
			}
		})
	}
}

// Every target is checked before the first write. When one profile cannot be
// applied, the others are not written either, and no backup is left.
func TestPolicyApply_ValidatesEveryTargetBeforeWriting(t *testing.T) {
	cases := []struct {
		name     string
		second   string // profile entry that cannot be applied
		sabotage func(t *testing.T, dir string)
		want     string
	}{
		{
			name:   "second target is a symlink",
			second: profileJSON("beta", "small", "Beta"),
			sabotage: func(t *testing.T, dir string) {
				link(t, filepath.Join(t.TempDir(), "elsewhere.md"), filepath.Join(dir, ".claude", "agents", "beta.md"))
			},
			want: "symlink",
		},
		{
			name:   "second target is a directory",
			second: profileJSON("beta", "small", "Beta"),
			sabotage: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, ".claude", "agents", "beta.md"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "not a regular file",
		},
		{
			name:     "second profile has no description",
			second:   profileJSON("beta", "small", ""),
			sabotage: func(*testing.T, string) {},
			want:     "description",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, routingPolicy(
				profileJSON("alpha", "small", "Alpha"),
				profileJSON("existing", "mid", ""),
				c.second,
			))
			// "existing" is a valid update, so a bad later target must hold back a backup too.
			existing := filepath.Join(dir, ".claude", "agents", "existing.md")
			const before = "---\nname: existing\ndescription: d\nmodel: old\n---\nbody\n"
			writeTestFile(t, existing, before, 0o644)
			c.sabotage(t, dir)

			code, _, stderr := runApply(t, dir, home)
			if code != 1 || !strings.Contains(stderr, c.want) || !strings.Contains(stderr, "nothing was written") {
				t.Fatalf("exit %d, stderr %q; want 1, %q and nothing was written", code, stderr, c.want)
			}
			if !missing(t, filepath.Join(dir, ".claude", "agents", "alpha.md")) {
				t.Error("alpha was written although beta could not be")
			}
			if got := readTestFile(t, existing); got != before {
				t.Errorf("existing was changed:\n%s", got)
			}
			if matches, _ := filepath.Glob(filepath.Join(dir, ".claude", "agents", "*.downshift-*")); len(matches) > 0 {
				t.Errorf("backup or temp files left behind: %v", matches)
			}
		})
	}
}

// An I/O failure in the middle of the write phase restores what the run had
// already written: here the second file lives in a read-only directory, so its
// backup cannot be created after the first file is in place.
func TestPolicyApply_WriteFailureRestoresEarlierWrites(t *testing.T) {
	skipWithoutUnixModes(t)
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir, home := setupPolicyApplyEnv(t, routingPolicy(
		profileJSON("alpha", "small", "Alpha"), // new file in the project
		profileJSON("beta", "small", ""),       // user-level agent, no project copy: updated in HOME
	))
	userAgents := filepath.Join(home, ".claude", "agents")
	const before = "---\nname: beta\ndescription: d\nmodel: old\n---\nbody\n"
	writeTestFile(t, filepath.Join(userAgents, "beta.md"), before, 0o644)
	if err := os.Chmod(userAgents, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(userAgents, 0o755) })

	code, stdout, stderr := runApply(t, dir, home)
	if code != 1 || !strings.Contains(stderr, "backup failed") || !strings.Contains(stderr, "restored") {
		t.Fatalf("exit %d, stderr %q; want 1, a backup failure and a restore", code, stderr)
	}
	if strings.Contains(stdout, "applied to") {
		t.Errorf("a file was reported as applied although the run failed:\n%s", stdout)
	}
	if !missing(t, filepath.Join(dir, ".claude", "agents", "alpha.md")) {
		t.Error("alpha was left written after beta failed")
	}
	if got := readTestFile(t, filepath.Join(userAgents, "beta.md")); got != before {
		t.Errorf("beta was changed:\n%s", got)
	}
}
