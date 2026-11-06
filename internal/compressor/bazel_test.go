// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"
	"testing"
)

const bazelBuild = "INFO: Analyzed target //app:main (45 packages loaded, 312 targets configured).\n" +
	"INFO: Found 1 target...\nTarget //app:main up-to-date:\n  bazel-bin/app/main\n" +
	"INFO: Elapsed time: 12.345s, Critical Path: 3.21s\nINFO: 25 processes: 12 internal, 13 linux-sandbox.\n" +
	"INFO: Build completed successfully, 25 total actions\n"

func TestCompressBazel(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string // substrings of the compacted output; nil means raw
	}{
		{"build", bazelBuild, []string{"[x] bazel: build completed successfully, 25 total actions in 12.345s", "Target //app:main up-to-date:", "  bazel-bin/app/main"}},
		{"test", "INFO: Analyzed 2 targets (0 packages loaded, 0 targets configured).\nINFO: Found 2 test targets...\n" +
			"INFO: Elapsed time: 4.2s, Critical Path: 2.10s\nINFO: 7 processes: 3 internal, 4 linux-sandbox.\nINFO: Build completed successfully, 7 total actions\n" +
			"//app:a_test                                  (cached) PASSED in 0.3s\n//app:b_test                                           PASSED in 1.1s\n\nExecuted 1 out of 2 tests: 2 tests pass.\n",
			[]string{"[x] bazel: build completed successfully, 7 total actions in 4.2s", "Executed 1 out of 2 tests: 2 tests pass."}},
		{"warning", strings.Replace(bazelBuild, "INFO: Found", "WARNING: x is deprecated\nINFO: Found", 1), nil},
		{"failed test", bazelBuild + "//app:a_test    FAILED in 0.3s\n", nil},
		{"error", "ERROR: /w/BUILD:1:1: compiling failed\n" + bazelBuild, nil},
		{"debug line", "DEBUG: /w/x.bzl:1:1: note\n" + bazelBuild, nil},
		{"unknown line", "Loading: 0 packages loaded\n" + bazelBuild, nil},
		{"danger word in a target", strings.Replace(bazelBuild, "//app:main up-to", "//app:error_main up-to", 1), nil},
		{"command echo", "bazel build //...\n" + bazelBuild, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Compress([]byte(tc.in), ModeSafe)
			if tc.want == nil {
				if res.Applied || string(res.Output) != tc.in {
					t.Fatalf("expected raw, got applied=%v %q", res.Applied, res.Output)
				}
				return
			}
			if !res.Applied || res.Format != FormatBazel || len(res.Output) >= len(tc.in) {
				t.Fatalf("not compacted: %+v", res)
			}
			for _, w := range tc.want {
				if !strings.Contains(string(res.Output), w) {
					t.Errorf("missing %q in %q", w, res.Output)
				}
			}
		})
	}
}
