// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import "testing"

const jestStderrClean = "PASS src/a.test.ts\nPASS src/b.test.ts\nPASS src/c.test.ts\n\nTest Suites: 3 passed, 3 total\nTests:       12 passed, 12 total\nSnapshots:   0 total\nTime:        1.234 s\nRan all test suites.\n"

// Jest writes its whole report to stderr, so a green run has empty stdout.
func TestRun_JestReportOnStderr(t *testing.T) {
	fail := "FAIL src/a.test.ts\n\nTest Suites: 1 failed, 1 total\nTests:       1 failed, 11 passed, 12 total\n"
	cases := []struct {
		name, raw string
		code      int
		compacted bool
	}{
		{"green stderr report compacts", jestStderrClean, 0, true},
		{"FAIL on stderr with exit 0 stays raw", fail, 0, false},
		{"failing run stays raw", fail, 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := "cat >&2 <<'EOF'\n" + tc.raw + "EOF\n"
			if tc.code != 0 {
				script += "exit 1\n"
			}
			fakeBin(t, "jest", script)
			r, out, errOut := run(t, []string{"jest"}, true)
			if r.ExitCode != tc.code || r.Compacted != tc.compacted {
				t.Fatalf("result=%+v", r)
			}
			if tc.compacted {
				if want := compactedWant(tc.raw); out != want || errOut != "" {
					t.Fatalf("stdout=%q stderr=%q, want %q", out, errOut, want)
				}
				return
			}
			if out != "" || errOut != tc.raw {
				t.Fatalf("raw must go back to stderr: stdout=%q stderr=%q", out, errOut)
			}
		})
	}
}

// Raw replay keeps each stream on its own writer.
func TestRun_JestMixedStreamsReplayRaw(t *testing.T) {
	fakeBin(t, "jest", "echo out1\necho err1 >&2\necho out2\nexit 1\n")
	r, out, errOut := run(t, []string{"jest"}, true)
	if r.Compacted || out != "out1\nout2\n" || errOut != "err1\n" {
		t.Fatalf("result=%+v stdout=%q stderr=%q", r, out, errOut)
	}
}

// A family whose reporter is not on stderr keeps the old rule: any stderr
// byte releases the output untouched.
func TestRun_OtherFamilyStderrStaysRaw(t *testing.T) {
	fakeBin(t, "vitest", "cat >&2 <<'EOF'\n"+jestStderrClean+"EOF\n")
	r, out, errOut := run(t, []string{"vitest"}, true)
	if r.Compacted || out != "" || errOut != jestStderrClean {
		t.Fatalf("result=%+v stdout=%q stderr=%q", r, out, errOut)
	}
}
