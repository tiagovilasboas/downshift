// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_KeepFailedStoresOnlyFailures(t *testing.T) {
	cases := []struct {
		name      string
		script    string
		keep      bool
		wantCalls int
		wantID    bool
	}{
		{"failure is stored", shellPrintf(failedSuite) + "; exit 1", false, 1, true},
		{"success is not stored", shellPrintf(passingSuite), false, 0, false},
		{"full Keep wins over KeepFailed", shellPrintf(passingSuite), true, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, "go", tc.script)
			calls := 0
			put := func(b []byte) (string, bool) { calls++; return "0123456789ab", true }
			var out, errb bytes.Buffer
			o := Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb, Compact: true, KeepFailed: put}
			wantCalls := tc.wantCalls
			if tc.keep {
				o.Keep, o.KeepFailed = put, func([]byte) (string, bool) { t.Fatal("KeepFailed used"); return "", false }
				wantCalls = 1
			}
			Run([]string{"go", "test", "./..."}, o)
			if calls != wantCalls {
				t.Fatalf("calls %d want %d", calls, wantCalls)
			}
			if got := strings.Contains(out.String(), "downshift retrieve 0123456789ab"); got != tc.wantID {
				t.Fatalf("id note %v want %v: %q", got, tc.wantID, out.String())
			}
		})
	}
}
