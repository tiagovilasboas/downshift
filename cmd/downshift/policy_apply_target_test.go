// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The agent files go under the directory that holds .downshift/policy.json,
// wherever in the project the command is started.
func TestPolicyApply_AnchorsToTheProjectRoot(t *testing.T) {
	cases := []struct {
		name  string
		start []string // path below the project root the command starts in
	}{
		{"at the root", nil},
		{"one level down", []string{"pkg"}},
		{"three levels down", []string{"a", "b", "c"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("scout", "small", "Scouts")))
			start := filepath.Join(append([]string{dir}, c.start...)...)
			if err := os.MkdirAll(start, 0o755); err != nil {
				t.Fatal(err)
			}

			code, stdout, stderr := runApply(t, start, home, func(o *policyApplyOptions) { o.Yes = true })
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			if missing(t, filepath.Join(dir, ".claude", "agents", "scout.md")) {
				t.Error("agent not written under the project root")
			}
			if start != dir && !missing(t, filepath.Join(start, ".claude")) {
				t.Errorf("agent written under the starting directory %s", start)
			}
			if !strings.Contains(stdout, "applied to .claude/agents/scout.md") {
				t.Errorf("output does not show the root-relative path:\n%s", stdout)
			}
		})
	}
}

// Without a project policy there is no root to anchor to: nothing is written
// under the working directory.
func TestPolicyApply_NoProjectPolicyWritesNothing(t *testing.T) {
	dir, home := t.TempDir(), t.TempDir()
	code, _, stderr := runApply(t, dir, home)
	if code != 1 || !strings.Contains(stderr, "policy init") {
		t.Errorf("exit %d, stderr %q; want 1 and a hint to run policy init", code, stderr)
	}
	if !missing(t, filepath.Join(dir, ".claude")) {
		t.Error("a .claude directory was created without a project policy")
	}
}

// A symlink at the file, the agents directory or .claude is never written
// through, with --yes or without. The link and what it points to stay as they were.
func TestPolicyApply_RefusesSymlinkedTargets(t *testing.T) {
	const original = "---\nname: scout\ndescription: real\nmodel: old\n---\nbody\n"
	cases := []struct {
		name string
		// setup builds the links under dir/home. realDir holds what they point to.
		setup func(t *testing.T, dir, home, realDir string)
		// untouched lists the files below realDir that must keep their content.
		untouched string
	}{
		{
			name: "file symlink in the project",
			setup: func(t *testing.T, dir, _, realDir string) {
				writeTestFile(t, filepath.Join(realDir, "scout.md"), original, 0o644)
				link(t, filepath.Join(realDir, "scout.md"), filepath.Join(dir, ".claude", "agents", "scout.md"))
			},
			untouched: "scout.md",
		},
		{
			name: "dangling file symlink",
			setup: func(t *testing.T, dir, _, realDir string) {
				link(t, filepath.Join(realDir, "gone.md"), filepath.Join(dir, ".claude", "agents", "scout.md"))
			},
		},
		{
			name: "agents directory is a symlink",
			setup: func(t *testing.T, dir, _, realDir string) {
				writeTestFile(t, filepath.Join(realDir, "scout.md"), original, 0o644)
				if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
					t.Fatal(err)
				}
				link(t, realDir, filepath.Join(dir, ".claude", "agents"))
			},
			untouched: "scout.md",
		},
		{
			name: ".claude is a symlink",
			setup: func(t *testing.T, dir, _, realDir string) {
				writeTestFile(t, filepath.Join(realDir, "agents", "scout.md"), original, 0o644)
				link(t, realDir, filepath.Join(dir, ".claude"))
			},
			untouched: filepath.Join("agents", "scout.md"),
		},
		{
			name: "file symlink in the user agents the project does not shadow",
			setup: func(t *testing.T, _, home, realDir string) {
				writeTestFile(t, filepath.Join(realDir, "scout.md"), original, 0o644)
				link(t, filepath.Join(realDir, "scout.md"), filepath.Join(home, ".claude", "agents", "scout.md"))
			},
			untouched: "scout.md",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("scout", "small", "Scouts")))
			realDir := t.TempDir()
			c.setup(t, dir, home, realDir)

			code, stdout, stderr := runApply(t, dir, home) // Yes is on
			if code != 1 {
				t.Fatalf("exit %d, want 1\nstdout: %s", code, stdout)
			}
			if !strings.Contains(stderr, "symlink") || !strings.Contains(stderr, "refusing to write through") {
				t.Errorf("stderr does not explain the refusal:\n%s", stderr)
			}
			if c.untouched != "" && readTestFile(t, filepath.Join(realDir, c.untouched)) != original {
				t.Error("the file behind the symlink was changed")
			}
			entries, _ := os.ReadDir(realDir)
			for _, e := range entries {
				if e.Name() != "scout.md" && e.Name() != "agents" {
					t.Errorf("unexpected file %s written behind the symlink", e.Name())
				}
			}
			if matches, _ := filepath.Glob(filepath.Join(dir, ".claude", "agents", "*.downshift-*")); len(matches) > 0 {
				t.Errorf("backup or temp files left behind: %v", matches)
			}
		})
	}
}

// A target that is a directory, not a file, is refused before anything is written.
func TestPolicyApply_RefusesNonRegularTarget(t *testing.T) {
	dir, home := setupPolicyApplyEnv(t, routingPolicy(profileJSON("scout", "small", "Scouts")))
	if err := os.MkdirAll(filepath.Join(dir, ".claude", "agents", "scout.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runApply(t, dir, home)
	if code != 1 || !strings.Contains(stderr, "not a regular file") {
		t.Errorf("exit %d, stderr %q; want 1 and not a regular file", code, stderr)
	}
}

func link(t *testing.T, target, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, name); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}
}
