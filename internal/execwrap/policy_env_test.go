// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDecide_EnvAssignments(t *testing.T) {
	cases := []struct {
		name    string
		command string
		rewrite bool
		want    string
		argv    string
		reason  string
	}{
		{"one assignment", "FOO=1 go test ./...", true, "FOO=1 '/opt/ds/downshift' compact -- go test ./...", "go test ./...", ReasonAllowlisted},
		{"two assignments", "A=1 B=2 go test ./...", true, "A=1 B=2 '/opt/ds/downshift' compact -- go test ./...", "go test ./...", ReasonAllowlisted},
		{"assignment with equals in value", "GOCOVERDIR=a=b go test ./...", true, "GOCOVERDIR=a=b '/opt/ds/downshift' compact -- go test ./...", "go test ./...", ReasonAllowlisted},
		{"empty value", "FOO= go test ./...", true, "FOO= '/opt/ds/downshift' compact -- go test ./...", "go test ./...", ReasonAllowlisted},
		{"git with assignment", "CI=1 git status", true, "CI=1 '/opt/ds/downshift' compact -- git status", "git status", ReasonAllowlisted},
		{"piped tail", "FOO=1 go test ./... | tail -5", true, "FOO=1 '/opt/ds/downshift' compact -- go test ./... | tail -5", "go test ./...", ReasonAllowlistedCompound},
		{"value with dollar", "FOO=$X go test", false, "", "", ReasonUnsafeChar},
		{"value with double quotes", `FOO="a b" go test`, false, "", "", ReasonUnsafeChar},
		{"value with backtick", "FOO=`id` go test", false, "", "", ReasonUnsafeChar},
		{"not allowlisted", "FOO=1 ls", false, "", "", ReasonNotAllowlisted},
		{"sudo after assignment", "FOO=1 sudo go test", false, "", "", ReasonNotAllowlisted},
		{"assignment only", "FOO=1", false, "", "", ReasonNotAllowlisted},
		{"name starts with digit", "1FOO=1 go test", false, "", "", ReasonNotAllowlisted},
		{"reserved downshift name", "DOWNSHIFT_EXEC=0 go test", false, "", "", ReasonNotAllowlisted},
		{"reserved git name", "GIT_DIR=/tmp/x git log", false, "", "", ReasonNotAllowlisted},
		{"reserved PATH", "PATH=/tmp/x go test", false, "", "", ReasonNotAllowlisted},
		{"already wrapped after assignment", "FOO=1 downshift compact -- go test", false, "", "", ReasonAlreadyWrapped},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite != tc.rewrite {
				t.Fatalf("Decide(%q).Rewrite = %v, want %v (%+v)", tc.command, d.Rewrite, tc.rewrite, d)
			}
			if d.Reason != tc.reason {
				t.Fatalf("Decide(%q).Reason = %q, want %q", tc.command, d.Reason, tc.reason)
			}
			if !tc.rewrite {
				return
			}
			if d.Command != tc.want {
				t.Fatalf("Command = %q, want %q", d.Command, tc.want)
			}
			if d.Argv != tc.argv {
				t.Fatalf("Argv = %q, want %q (permission rules match without the assignments)", d.Argv, tc.argv)
			}
		})
	}
}

// The rewritten command must still run its child with the assignment in
// effect and return the child's exit code. A POSIX shell is required.
func TestDecide_EnvAssignmentsRunThroughShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell required")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "downshift")
	script := "#!/bin/sh\nprintf '%s|%s\\n' \"$FOO\" \"$*\"\nexit 7\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	d := Decide("FOO=bar go test ./...", fake)
	if !d.Rewrite {
		t.Fatalf("not rewritten: %+v", d)
	}
	out, err := exec.Command("sh", "-c", d.Command).CombinedOutput()
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 7 {
		t.Fatalf("exit = %v, want 7 (out %q)", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != "bar|compact -- go test ./..." {
		t.Fatalf("child saw %q, want the assignment in effect", got)
	}
}
