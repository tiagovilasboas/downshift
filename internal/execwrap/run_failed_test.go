// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// failedSuite is ten passing packages and one failing test.
var failedSuite = passingSuite +
	"--- FAIL: TestCharge (0.01s)\n    billing_test.go:42: got 100, want 110\nFAIL\nFAIL\tgithub.com/x/internal/billing\t0.087s\nFAIL\n"

func shellPrintf(s string) string {
	return "printf '" + strings.ReplaceAll(s, "\t", "\\t") + "'"
}

func TestRun_FailedGoTestCompactsAndKeepsExitCode(t *testing.T) {
	fakeBin(t, "go", shellPrintf(failedSuite)+"; exit 1")
	r, out, errOut := run(t, []string{"go", "test", "./..."}, true)
	if r.ExitCode != 1 || r.ExitClass != ExitClassNonZero || !r.Compacted || errOut != "" {
		t.Fatalf("code=%d class=%s compacted=%v stderr=%q", r.ExitCode, r.ExitClass, r.Compacted, errOut)
	}
	body := compressor.CompressFailed([]byte(failedSuite), compressor.FormatGoTest).Output
	if want := string(body) + Marker(len(failedSuite), len(body)); out != want {
		t.Fatalf("output\n got: %q\nwant: %q", out, want)
	}
	for _, l := range []string{"--- FAIL: TestCharge (0.01s)\n", "    billing_test.go:42: got 100, want 110\n", "FAIL\tgithub.com/x/internal/billing\t0.087s\n"} {
		if !strings.Contains(out, l) {
			t.Errorf("lost failure line %q", l)
		}
	}
}

func TestRun_FailedGoTestKillSwitchAndStderrStayRaw(t *testing.T) {
	fakeBin(t, "go", shellPrintf(failedSuite)+"; exit 1")
	if r, out, _ := run(t, []string{"go", "test"}, false); r.Compacted || out != failedSuite || r.ExitCode != 1 {
		t.Fatalf("kill switch: compacted=%v code=%d", r.Compacted, r.ExitCode)
	}
	fakeBin(t, "go", shellPrintf(failedSuite)+"; echo oops >&2; exit 1")
	if r, out, errOut := run(t, []string{"go", "test"}, true); r.Compacted || out != failedSuite || errOut != "oops\n" || r.ExitCode != 1 {
		t.Fatalf("stderr: compacted=%v code=%d", r.Compacted, r.ExitCode)
	}
}

func TestRun_FailedRunOfOtherFamiliesStaysRaw(t *testing.T) {
	fakeBin(t, "git", "printf 'On branch main\\nnothing to commit, working tree clean\\n'; exit 1")
	if r, out, _ := run(t, []string{"git", "status"}, true); r.Compacted || r.ExitCode != 1 || !strings.HasPrefix(out, "On branch main\nnothing") {
		t.Fatalf("git status failed run changed: compacted=%v out=%q", r.Compacted, out)
	}
}
