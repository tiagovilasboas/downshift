// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

// The fixtures are hand-written (deno and zig were not installed where they
// were made), so the fake binary replays them verbatim.
func TestRun_DenoAndZigCleanCompactsWarningAndFailureStayRaw(t *testing.T) {
	cases := []struct {
		bin  string
		argv []string
		file string
		warn string
	}{
		{"deno", []string{"deno", "check", "main.ts"}, "deno_check_many_files_clean.txt", "deno_test_summary_preserve.txt"},
		{"zig", []string{"zig", "build"}, "zig_build_summary_clean.txt", "zig_build_warning_preserve.txt"},
	}
	for _, c := range cases {
		clean := auditFixture(t, c.file)
		fakeBin(t, c.bin, "cat <<'EOF'\n"+clean+"EOF")
		r, out, _ := run(t, c.argv, true)
		if r.ExitCode != 0 || len(out) >= len(clean) || !strings.Contains(out, "lines elided]") {
			t.Fatalf("%s clean: exit=%d len=%d/%d", c.bin, r.ExitCode, len(out), len(clean))
		}
		warn := auditFixture(t, c.warn)
		fakeBin(t, c.bin, "cat <<'EOF'\n"+warn+"EOF")
		if _, out, _ := run(t, c.argv, true); out != warn {
			t.Fatalf("%s warning run changed: %q", c.bin, out)
		}
		fakeBin(t, c.bin, "cat <<'EOF'\n"+clean+"EOF\nexit 1")
		if r, out, _ := run(t, c.argv, true); r.ExitCode != 1 || out != clean {
			t.Fatalf("%s failed run: exit=%d changed=%v", c.bin, r.ExitCode, out != clean)
		}
	}
}
