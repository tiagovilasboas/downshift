// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun_NotFound127(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r, out, errOut := run(t, []string{"go", "test"}, true)
	if r.ExitCode != ExitNotFound || r.ExitClass != ExitClassNotFound {
		t.Fatalf("exit = %d/%s, want 127/not_found", r.ExitCode, r.ExitClass)
	}
	if out != "" || !strings.Contains(errOut, "go: command not found") {
		t.Fatalf("stdout=%q stderr=%q", out, errOut)
	}
}

// exec is not a generic runner: anything outside the allowlist is refused
// with 126 and nothing is spawned (the marker file would exist otherwise).
func TestRun_RefusesNonAllowlisted(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	fakeBin(t, "tool", "touch "+marker)
	fakeBin(t, "downshift", "touch "+marker)
	for _, argv := range [][]string{
		{"tool"},
		{"rm", "-rf", marker},
		{"downshift", "exec", "--", "go", "test"}, // recursion
		{"/usr/bin/git", "status"},
		{"git", "--version"},
		{"git", "-c", "core.pager=x", "log"},
		{"sudo", "rm", "-rf", "/"},
		{"eval", "echo", "test"},
		{"go"},
	} {
		r, out, errOut := run(t, argv, true)
		if r.ExitCode != ExitCannotExecute || r.ExitClass != ExitClassRefused || r.Family != FamilyNone {
			t.Fatalf("%v: result = %+v", argv, r)
		}
		if out != "" || !strings.Contains(errOut, "refused") {
			t.Fatalf("%v: stdout=%q stderr=%q", argv, out, errOut)
		}
		if _, err := os.Stat(marker); err == nil {
			t.Fatalf("%v: a program was spawned", argv)
		}
	}
}

func TestRun_NoArgv(t *testing.T) {
	r, _, errOut := run(t, nil, true)
	if r.ExitCode != 2 || errOut == "" {
		t.Fatalf("exit = %d stderr=%q", r.ExitCode, errOut)
	}
}
