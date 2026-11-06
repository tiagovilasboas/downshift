// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func bigOutputFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "big_output", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Large clean runs that used to pass through now compact end to end: a
// multi-package go test -v, go test -json, and a package script whose runner
// logged "Error:" in a passing test.
func TestRun_BigCleanTestOutputCompacts(t *testing.T) {
	cases := []struct {
		name, bin string
		argv      []string
		fixture   string
		want      []string
	}{
		{"go test -v ./...", "go", []string{"go", "test", "-v", "./..."}, "go_v_multi_clean.in",
			[]string{"Go test -v: PASS (5 packages ok", "retry after timeout, second attempt ok"}},
		{"go test -json", "go", []string{"go", "test", "-json", "./..."}, "go_json_clean.in",
			[]string{"Go test -json: PASS (3 packages ok, 30 passed"}},
		{"yarn test", "yarn", []string{"yarn", "test"}, "yarn_jest_pass.in",
			[]string{"Jest: PASS (88 passed across 20 suites)", "Error: simulated network failure (expected)"}},
		{"pnpm test", "pnpm", []string{"pnpm", "test"}, "pnpm_ws_vitest_pass.in",
			[]string{"Workspace: packages/web test", "Vitest: PASS (150 passed"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := bigOutputFixture(t, tc.fixture)
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+raw+"EOF\n")
			r, out, errOut := run(t, tc.argv, true)
			if r.ExitCode != 0 || !r.Compacted || errOut != "" {
				t.Fatalf("result=%+v stderr=%q", r, errOut)
			}
			for _, w := range tc.want {
				if !strings.Contains(out, w) {
					t.Errorf("output lacks %q:\n%s", w, out)
				}
			}
			if len(out)*3 > len(raw) {
				t.Errorf("saving below 67%%: %d -> %d", len(raw), len(out))
			}
		})
	}
}
