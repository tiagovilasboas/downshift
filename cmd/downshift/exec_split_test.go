// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// fakeGoOnPath puts a `go` script on PATH that prints a line on stdout and on
// stderr and exits with $FAKE_EXIT.
func fakeGoOnPath(t *testing.T) {
	t.Helper()
	bin := t.TempDir()
	script := "#!/bin/sh\necho out-line\necho err-line >&2\nexit \"${FAKE_EXIT:-0}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// A single quoted command word runs as the split argv: the exit code is the
// program's own, and failing output stays raw (byte-identical, both streams).
func TestRunExec_SingleQuotedWordKeepsExitCode(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv(execwrap.DepthEnv, "")
	fakeGoOnPath(t)
	cases := []struct {
		name, exit string
		wantRC     int
	}{
		{"clean exit 0", "0", 0},
		{"failure exit 3", "3", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FAKE_EXIT", tc.exit)
			var out, errb bytes.Buffer
			rc := runExec([]string{"--", "go test ./secret/..."}, strings.NewReader(""), &out, &errb)
			if rc != tc.wantRC {
				t.Fatalf("rc=%d want %d (stderr=%q)", rc, tc.wantRC, errb.String())
			}
			if tc.wantRC != 0 && !strings.Contains(out.String(), "out-line") {
				t.Fatalf("failing run lost output: %q", out.String())
			}
		})
	}
}

// A quoted word with a metacharacter is not split and is refused, as before.
func TestRunExec_SingleQuotedWordWithMetacharacterRefused(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	var out, errb bytes.Buffer
	rc := runExec([]string{"--", "go test ./... | tail"}, strings.NewReader(""), &out, &errb)
	if rc != execwrap.ExitCannotExecute {
		t.Fatalf("rc=%d want %d (stderr=%q)", rc, execwrap.ExitCannotExecute, errb.String())
	}
}
