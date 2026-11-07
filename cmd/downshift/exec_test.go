// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestRunExec_ExitCodeAndPromptFreeLog(t *testing.T) {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	t.Setenv(execwrap.DepthEnv, "") // clear in case we're running inside `downshift compactor`
	t.Setenv("DOWNSHIFT_CCR", "0")  // this test pins the plain marker; CCR has its own test below
	rawSuite := fakeGoTestSuite(t)

	compacted := compressor.CompressExit([]byte(rawSuite), compressor.ModeSafe, 0).Output
	cases := []struct {
		name, exit, killSwitch string
		wantRC                 int
		wantOut                string
	}{
		{"clean run compacts", "0", "", 0, string(compacted) + execwrap.Marker(len(rawSuite), len(compacted))},
		{"non-zero exit stays raw and keeps the code", "1", "", 1, rawSuite},
		{"kill switch keeps raw", "0", "0", 0, rawSuite},
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

// fakeGoTestSuite puts a fake `go` first on PATH that prints eight passing
// packages and exits with $FAKE_EXIT, and returns what it prints.
func fakeGoTestSuite(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	var suite strings.Builder
	for i := 0; i < 8; i++ {
		fmt.Fprintf(&suite, "ok  \tsecret/pkg/p%d\t0.1s\n", i)
	}
	rawSuite := suite.String()
	if err := os.WriteFile(filepath.Join(bin, "suite.txt"), []byte(rawSuite), 0o600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat \"$(dirname \"$0\")/suite.txt\"\nexit \"${FAKE_EXIT:-0}\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return rawSuite
}

// CCR is on by default: a compacted run names an id, and `retrieve` returns the
// raw output byte for byte. DOWNSHIFT_CCR=0, "ccr": false and a config file that
// cannot be read each leave the plain marker and store nothing.
func TestRunExec_CCROnByDefault(t *testing.T) {
	cases := []struct {
		name, env, config string
		wantKept          bool
	}{
		{"nothing set", "", "", true},
		{"env on over file off", "1", `{"ccr": false}`, true},
		{"env off", "0", "", false},
		{"file off", "", `{"ccr": false}`, false},
		{"unreadable file", "", `{bad`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := t.TempDir()
			t.Setenv(paths.EnvStateDir, state)
			t.Setenv(execwrap.DepthEnv, "")
			t.Setenv("DOWNSHIFT_EXEC", "")
			t.Setenv("FAKE_EXIT", "0")
			t.Setenv("DOWNSHIFT_CCR", tc.env)
			rawSuite := fakeGoTestSuite(t)
			if tc.config != "" {
				cfg := filepath.Join(state, "context-optimization.json")
				if err := os.WriteFile(cfg, []byte(tc.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			var out, errb bytes.Buffer
			if rc := runExec([]string{"--", "go", "test", "./secret/..."}, strings.NewReader(""), &out, &errb); rc != 0 {
				t.Fatalf("rc=%d stderr=%q", rc, errb.String())
			}
			entries, _ := os.ReadDir(filepath.Join(state, "ccr"))
			if !tc.wantKept {
				if len(entries) != 0 || strings.Contains(out.String(), "downshift retrieve") {
					t.Fatalf("CCR stored or named an id while off: %d entries, out=%q", len(entries), out.String())
				}
				return
			}
			if len(entries) != 1 {
				t.Fatalf("want 1 kept entry, got %d; out=%q", len(entries), out.String())
			}
			id := strings.TrimSuffix(entries[0].Name(), ".txt")
			if !strings.Contains(out.String(), "full output: downshift retrieve "+id) {
				t.Fatalf("marker does not name %s: %q", id, out.String())
			}
			if code, got, errs := retrieve(id); code != 0 || got != rawSuite {
				t.Fatalf("retrieve rc=%d err=%q\n got %q\nwant %q", code, errs, got, rawSuite)
			}
		})
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
