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

func TestRunExec_ExitCodeAndPromptFreeLog(t *testing.T) {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'ok  \\tsecret/pkg/a\\t0.1s\\nok  \\tsecret/pkg/b\\t0.1s\\n'\nexit \"${FAKE_EXIT:-0}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	cases := []struct {
		name, exit, killSwitch string
		wantRC                 int
		wantOut                string
	}{
		{"clean run compacts", "0", "", 0, "Go test: PASS (2 packages ok)\n"},
		{"non-zero exit stays raw and keeps the code", "1", "", 1, "ok  \tsecret/pkg/a\t0.1s\nok  \tsecret/pkg/b\t0.1s\n"},
		{"kill switch keeps raw", "0", "0", 0, "ok  \tsecret/pkg/a\t0.1s\nok  \tsecret/pkg/b\t0.1s\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FAKE_EXIT", tc.exit)
			t.Setenv("DOWNSHIFT_EXEC", tc.killSwitch)
			var out, errb bytes.Buffer
			rc := runExec([]string{"--", "go", "test", "./secret/..."}, strings.NewReader(""), &out, &errb)
			if rc != tc.wantRC || out.String() != tc.wantOut {
				t.Fatalf("rc=%d out=%q stderr=%q", rc, out.String(), errb.String())
			}
		})
	}

	data, err := os.ReadFile(filepath.Join(state, execwrap.EventsFileName))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "\n"); n != 3 {
		t.Fatalf("want 3 exec records, got %d:\n%s", n, data)
	}
	for _, leaked := range []string{"secret", "./", "pkg", "ok "} {
		if strings.Contains(string(data), leaked) {
			t.Fatalf("exec log leaked %q:\n%s", leaked, data)
		}
	}
	s, err := execwrap.Summarize(filepath.Join(state, execwrap.EventsFileName))
	if err != nil || s.Compacted != 1 || s.SavedBytes <= 0 {
		t.Fatalf("summary = %+v err=%v", s, err)
	}
}

func TestRunExec_Usage(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	var out, errb bytes.Buffer
	for _, args := range [][]string{nil, {"go", "test"}} {
		if rc := runExec(args, strings.NewReader(""), &out, &errb); rc != 2 {
			t.Fatalf("args %v: rc=%d, want 2", args, rc)
		}
	}
	if rc := runExec([]string{"--"}, strings.NewReader(""), &out, &errb); rc != 2 {
		t.Fatalf("empty argv: rc=%d, want 2", rc)
	}
}
