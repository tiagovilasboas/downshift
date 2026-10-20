// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

const compactRenameExe = "/opt/ds/downshift"

// The shell wrapper is named `compact`; `exec` is only an alias for commands
// rewritten by older versions.
func TestCompactRename_RewriteUsesCompactName(t *testing.T) {
	cases := []struct {
		name    string
		command string
	}{
		{"go test", "go test ./..."},
		{"git status", "git status"},
		{"pytest", "pytest -v"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := execwrap.Decide(tc.command, compactRenameExe)
			if !d.Rewrite {
				t.Fatalf("Decide(%q) did not rewrite: reason=%s", tc.command, d.Reason)
			}
			if !strings.Contains(d.Command, " compact -- ") {
				t.Errorf("wrapped command %q lacks \" compact -- \"", d.Command)
			}
			if strings.Contains(d.Command, " exec -- ") {
				t.Errorf("wrapped command %q still uses \" exec -- \"", d.Command)
			}
		})
	}
}

// Both names must count as already wrapped, so a rewritten command is never
// wrapped again.
func TestCompactRename_AlreadyWrappedForBothNames(t *testing.T) {
	cases := []struct {
		name    string
		command string
	}{
		{"compact", "downshift compact -- go test ./..."},
		{"exec alias", "downshift exec -- go test ./..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := execwrap.Decide(tc.command, compactRenameExe)
			if d.Rewrite {
				t.Fatalf("Decide(%q) rewrote an already wrapped command", tc.command)
			}
			if d.Reason != execwrap.ReasonAlreadyWrapped {
				t.Errorf("reason = %q, want %q", d.Reason, execwrap.ReasonAlreadyWrapped)
			}
		})
	}
}

// The runner's messages name `compact`, not `exec`.
func TestCompactRename_RunnerMessageNamesCompact(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rc := runExec([]string{"--"}, strings.NewReader(""), &stdout, &stderr)
	if rc != 2 {
		t.Errorf("exit code = %d, want 2", rc)
	}
	msg := stderr.String()
	if !strings.Contains(msg, "downshift compact:") {
		t.Errorf("stderr %q does not name \"downshift compact:\"", msg)
	}
	if strings.Contains(msg, "downshift exec") {
		t.Errorf("stderr %q still names \"downshift exec\"", msg)
	}
}
