// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSTestRunner(t *testing.T) {
	read := func(parts ...string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(append([]string{"testdata"}, parts...)...))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	cases := []struct {
		name string
		in   string
		want Format
	}{
		{"npm vitest pass", read("script_runs", "npm_vitest_v3_pass.in"), FormatVitest},
		{"npm vitest fail", read("script_runs", "npm_vitest_v3_fail.in"), FormatVitest},
		{"pnpm vitest fail", read("corpus", "pnpm_test_fail.txt"), FormatVitest},
		{"coloured vitest fail", read("failed_vitest", "v4_fail_colour.in"), FormatVitest},
		{"npm jest pass", read("script_runs", "npm_jest_pass.in"), FormatJest},
		{"jest fail", read("corpus", "jest_fail.txt"), FormatJest},
		{"both runners", "Test Files  1 failed (1)\nTest Suites: 1 failed, 1 total\n", FormatUnknown},
		{"neither runner", "FAIL src/a.test.js\nTests: 1 failed, 1 total\n", FormatUnknown},
		{"no final newline", "Test Suites: 1 failed, 1 total", FormatUnknown},
		{"empty", "", FormatUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := JSTestRunner([]byte(tc.in)); got != tc.want {
				t.Fatalf("JSTestRunner = %q, want %q", got, tc.want)
			}
		})
	}
}
