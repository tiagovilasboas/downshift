// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

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

// TestRunExec_AllowlistedQuery covers `downshift compact --allowlisted -- <argv>`:
// exit 0 for an allowlisted family, 1 for none, 2 without the `--`; nothing
// printed, nothing run, no exec log row.
func TestRunExec_AllowlistedQuery(t *testing.T) {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	cases := []struct {
		name   string
		args   []string
		wantRC int
	}{
		{"go test is allowlisted", []string{"--allowlisted", "--", "go", "test", "./..."}, 0},
		{"git status is allowlisted", []string{"--allowlisted", "--", "git", "status"}, 0},
		{"docker run is not", []string{"--allowlisted", "--", "docker", "run", "alpine"}, 1},
		{"git -C is not", []string{"--allowlisted", "--", "git", "-C", "/tmp", "status"}, 1},
		{"npm --version is not", []string{"--allowlisted", "--", "npm", "--version"}, 1},
		{"one quoted word is split", []string{"--allowlisted", "--", "go test ./..."}, 0},
		{"one quoted word not allowlisted", []string{"--allowlisted", "--", "rm -rf x"}, 1},
		{"one quoted word with a pipe stays refused", []string{"--allowlisted", "--", "go test ./... | tail"}, 1},
		{"empty argv is not", []string{"--allowlisted", "--"}, 1},
		{"no -- is usage", []string{"--allowlisted", "go", "test"}, 2},
		{"flag alone is usage", []string{"--allowlisted"}, 2},
		{"after --session", []string{"--session=" + strings.Repeat("a", 16), "--allowlisted", "--", "go", "test"}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			rc := runExec(tc.args, strings.NewReader(""), &out, &errb)
			if rc != tc.wantRC {
				t.Fatalf("rc=%d want %d (stderr=%q)", rc, tc.wantRC, errb.String())
			}
			if out.Len() != 0 {
				t.Fatalf("query printed to stdout: %q", out.String())
			}
			if rc != 2 && errb.Len() != 0 {
				t.Fatalf("query printed to stderr: %q", errb.String())
			}
		})
	}
	if _, err := os.Stat(filepath.Join(state, execwrap.EventsFileName)); !os.IsNotExist(err) {
		t.Fatalf("query wrote an exec log row (stat err=%v)", err)
	}
}
