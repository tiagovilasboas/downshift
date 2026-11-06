// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

// Each wired family: a clean transcript compacts, a run with a failure stays
// whole with the program's exit code, and a clean-looking transcript with a
// problem line mixed in stays whole. The fixtures are the compressor corpus.
func TestRun_WiredFamiliesCompactCleanAndKeepFailures(t *testing.T) {
	cases := []struct {
		bin      string
		argv     []string
		clean    string
		failing  string
		mustKeep string
	}{
		{"rspec", []string{"rspec"}, "rspec_pass_clean.txt", "rspec_fail.txt", "expected: 110"},
		{"dotnet", []string{"dotnet", "test"}, "dotnet_test_pass_clean.txt", "dotnet_test_fail.txt", "Expected 110, Actual 100"},
		{"rubocop", []string{"rubocop"}, "rubocop_clean.txt", "", ""},
		{"rubocop", []string{"rubocop"}, "rubocop_offenses.txt", "", ""},
		{"phpstan", []string{"phpstan", "analyse"}, "phpstan_clean.txt", "", ""},
		{"docker", []string{"docker", "build", "-t", "api", "."}, "docker_build_clean.txt", "", ""},
		{"docker", []string{"docker", "ps"}, "docker_ps_clean.txt", "docker_ps_quiet_ids.txt", ""},
		{"docker", []string{"docker", "images"}, "docker_images_clean.txt", "docker_images_quiet_ids.txt", ""},
		{"kubectl", []string{"kubectl", "get", "pods"}, "kubectl_pods_clean.txt", "kubectl_get_services.txt", ""},
		{"git", []string{"git", "stash", "list"}, "stash_list_clean.txt", "stash_push_saved.txt", ""},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.argv, " "), func(t *testing.T) {
			clean := corpusFixture(t, tc.clean)
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+clean+"EOF\n")
			r, out, _ := run(t, tc.argv, true)
			if r.ExitCode != 0 || !r.Compacted || len(out) >= len(clean) {
				t.Fatalf("clean run not compacted: result=%+v len=%d/%d", r, len(out), len(clean))
			}

			// A problem line inside an otherwise clean transcript keeps it whole.
			noisy := clean + "unexpected error in output\n"
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+noisy+"EOF\n")
			if r, out, _ := run(t, tc.argv, true); r.Compacted || out != noisy {
				t.Fatalf("a transcript with a problem line must stay whole: compacted=%v", r.Compacted)
			}

			// A non-zero exit passes through byte for byte with its code.
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+clean+"EOF\nexit 3\n")
			if r, out, _ := run(t, tc.argv, true); r.ExitCode != 3 || r.Compacted || out != clean {
				t.Fatalf("failed run: exit=%d compacted=%v changed=%v", r.ExitCode, r.Compacted, out != clean)
			}

			if tc.failing == "" {
				return
			}
			fail := corpusFixture(t, tc.failing)
			fakeBin(t, tc.bin, "cat <<'EOF'\n"+fail+"EOF\n")
			r, out, _ = run(t, tc.argv, true)
			if r.Compacted || out != fail {
				t.Fatalf("%s must pass through whole: compacted=%v", tc.failing, r.Compacted)
			}
			if tc.mustKeep != "" && !strings.Contains(out, tc.mustKeep) {
				t.Fatalf("lost failure detail %q", tc.mustKeep)
			}
		})
	}
}
