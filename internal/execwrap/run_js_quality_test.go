// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

// The fixtures are hand-written generic logs; the fake binary replays them.
func TestRun_JSQualityCleanCompactsWarningAndFailureStayRaw(t *testing.T) {
	cases := []struct {
		bin  string
		argv []string
	}{
		{"npm", []string{"npm", "run", "lint"}},
		{"knip", []string{"knip"}},
	}
	clean := auditFixture(t, "deno_check_many_files_clean.txt")
	warn := auditFixture(t, "deno_test_summary_preserve.txt")
	for _, c := range cases {
		fakeBin(t, c.bin, "cat <<'EOF'\n"+clean+"EOF")
		r, out, _ := run(t, c.argv, true)
		if r.ExitCode != 0 || len(out) >= len(clean) || !strings.Contains(out, "lines elided]") {
			t.Fatalf("%s clean: exit=%d len=%d/%d", c.bin, r.ExitCode, len(out), len(clean))
		}
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
