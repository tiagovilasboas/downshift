// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunShellInit_PrintsScript(t *testing.T) {
	var out, errw bytes.Buffer
	rc := runShellInit(nil, &out, &errw)
	if rc != 0 {
		t.Errorf("exit code = %d, want 0", rc)
	}
	if errw.Len() > 0 {
		t.Errorf("stderr = %q, want empty", errw.String())
	}
	script := out.String()

	// Must contain the wrapper function
	if !strings.Contains(script, "__downshift_wrap") {
		t.Error("missing __downshift_wrap function")
	}

	// Must define key aliases
	for _, cmd := range []string{"go()", "cargo()", "npm()", "git()"} {
		if !strings.Contains(script, cmd) {
			t.Errorf("missing alias for %s", cmd)
		}
	}

	// Must have the kill switch check
	if !strings.Contains(script, "DOWNSHIFT_SHELL_ALIASES") {
		t.Error("missing DOWNSHIFT_SHELL_ALIASES kill switch")
	}

	// Must check for terminal to skip interactive sessions
	if !strings.Contains(script, "-t 1") {
		t.Error("missing terminal check (-t 1)")
	}
}

func TestRunShellInit_Help(t *testing.T) {
	var out, errw bytes.Buffer
	rc := runShellInit([]string{"--help"}, &out, &errw)
	if rc != 0 {
		t.Errorf("exit code = %d, want 0", rc)
	}
	if !strings.Contains(out.String(), "enable") {
		t.Error("help should mention enable")
	}
}
