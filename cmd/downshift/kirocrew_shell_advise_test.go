// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// kirocrewShell runs the registered KiroCrew hook on one shell call, named as
// the live payload names it.
func kirocrewShell(t *testing.T, command string, env map[string]string) (rc int, stdout, stderr string) {
	t.Helper()
	return kirocrewShellTool(t, "shell", command, env)
}

func kirocrewShellTool(t *testing.T, tool, command string, env map[string]string) (rc int, stdout, stderr string) {
	t.Helper()
	withExec(t, true, env, true)
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", t.TempDir()+"/events.jsonl")
	stdin := `{"tool_name":"` + tool + `","tool_input":{"command":"` + command + `"}}`
	stdout = captureStdout(func() {
		stderr = captureStderr(func() {
			rc = harnessRegistry["kirocrew"].Hook(strings.NewReader(stdin), cmdCat)
		})
	})
	return rc, stdout, stderr
}

// A compressing family on KiroCrew: exit 2 and the wrapped command on stderr.
func TestKiroCrewShellAdvise_CompressingFamilyExits2(t *testing.T) {
	rc, stdout, stderr := kirocrewShell(t, "go test ./...", nil)
	if rc != 2 {
		t.Fatalf("rc = %d, want 2\nstderr:\n%s", rc, stderr)
	}
	if stdout != "" {
		t.Errorf("advise mode prints nothing on stdout, got %q", stdout)
	}
	if !strings.Contains(stderr, "compact") || !strings.Contains(stderr, "go test ./...") {
		t.Errorf("stderr does not carry the wrapped command: %q", stderr)
	}
}

// The older tool name still counts; an unrelated tool is never a shell call.
func TestKiroCrewShellAdvise_ToolNames(t *testing.T) {
	if rc, _, _ := kirocrewShellTool(t, "execute_bash", "go test ./...", nil); rc != 2 {
		t.Errorf("execute_bash rc = %d, want 2", rc)
	}
	if rc, _, _ := kirocrewShellTool(t, "fs_read", "go test ./...", nil); rc != 0 {
		t.Errorf("fs_read rc = %d, want 0", rc)
	}
}

// Families that do not pay for a retry, and every case that must not touch
// the command, stay exit 0 with nothing printed.
func TestKiroCrewShellAdvise_AllowsEverythingElse(t *testing.T) {
	cases := []struct {
		name, command string
		env           map[string]string
	}{
		{"git status is small", "git status", nil},
		{"git diff is small", "git diff", nil},
		{"not allowlisted", "echo hello", nil},
		{"kill switch", "go test ./...", map[string]string{"DOWNSHIFT_EXEC": "0"}},
		{"already inside a wrapper", "go test ./...", map[string]string{"DOWNSHIFT_EXEC_DEPTH": "1"}},
		{"already wrapped", testExe + " compact -- go test ./...", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rc, stdout, stderr := kirocrewShell(t, c.command, c.env)
			if rc != 0 || stdout != "" || strings.Contains(stderr, "Run it again") {
				t.Errorf("rc=%d stdout=%q stderr=%q, want a silent allow", rc, stdout, stderr)
			}
		})
	}
}
