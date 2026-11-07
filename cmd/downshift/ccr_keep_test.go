// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func fakeGo(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func goSuite(fail bool) string {
	var b strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		b.WriteString("ok  \\tgithub.com/x/internal/" + p + "\\t0.1s\\n")
	}
	if fail {
		b.WriteString("--- FAIL: TestCharge (0.01s)\\n    billing_test.go:42: got 100, want 110\\nFAIL\\nFAIL\\tgithub.com/x/internal/billing\\t0.087s\\nFAIL\\n")
	}
	return "printf '" + b.String() + "'"
}

var idNote = regexp.MustCompile(`downshift retrieve ([0-9a-f]{12})\]`)

func ccrTestEnv(t *testing.T) string {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	t.Setenv("DOWNSHIFT_EXEC", "")
	t.Setenv(execwrap.DepthEnv, "")
	return state
}

// CCR is part of the compactor: every compacted run is stored, success or
// failure, with no setting involved. The switches earlier versions read
// (DOWNSHIFT_CCR, DOWNSHIFT_CCR_FAILURES, "ccr", "ccr_failures") do nothing, and
// a config file that still has the keys loads without error. Only turning the
// compactor itself off stores nothing, because nothing is cut.
func TestCompactAlwaysKeepsTheRawOutput(t *testing.T) {
	cases := []struct {
		name   string
		fail   bool
		env    map[string]string
		config string
		wantID bool
	}{
		{"failure is stored", true, nil, "", true},
		{"success is stored", false, nil, "", true},
		{"old env off switch is ignored", true, map[string]string{"DOWNSHIFT_CCR_FAILURES": "0"}, "", true},
		{"old CCR=0 is ignored", false, map[string]string{"DOWNSHIFT_CCR": "0"}, "", true},
		{"old config off keys are ignored", false, nil, `{"ccr":false,"ccr_failures":false}`, true},
		{"exec kill switch stores nothing", true, map[string]string{"DOWNSHIFT_EXEC": "0"}, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := ccrTestEnv(t)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			if tc.config != "" {
				if err := os.WriteFile(filepath.Join(state, "context-optimization.json"), []byte(tc.config), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			body := goSuite(tc.fail)
			if tc.fail {
				body += "; exit 1"
			}
			fakeGo(t, body)
			var out, errb bytes.Buffer
			code := runExec([]string{"--", "go", "test", "./..."}, strings.NewReader(""), &out, &errb)
			if tc.fail != (code == 1) {
				t.Fatalf("exit %d", code)
			}
			m := idNote.FindStringSubmatch(out.String())
			if (m != nil) != tc.wantID {
				t.Fatalf("id note present=%v want %v: %q", m != nil, tc.wantID, out.String())
			}
			if m == nil {
				return
			}
			var rout, rerr bytes.Buffer
			if c := runRetrieve([]string{m[1], "--grep", "TestCharge"}, &rout, &rerr); c != 0 {
				t.Fatalf("retrieve exit %d: %s", c, rerr.String())
			}
			if tc.fail && !strings.Contains(rout.String(), "TestCharge") {
				t.Fatalf("retrieve lost the failure: %q", rout.String())
			}
		})
	}
}

// A file read of a secret path is never held, so nothing is stored.
func TestCompactDoesNotKeepSecretPathReads(t *testing.T) {
	state := ccrTestEnv(t)
	secret := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(secret, []byte(strings.Repeat("API_TOKEN=abc\n", 400)), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	runExec([]string{"--", "cat", secret}, strings.NewReader(""), &out, &errb)
	if idNote.MatchString(out.String()) {
		t.Fatalf("secret path output was stored: %q", out.String())
	}
	if entries, _ := os.ReadDir(filepath.Join(state, "ccr")); len(entries) != 0 {
		t.Fatalf("ccr store has %d entries", len(entries))
	}
}
