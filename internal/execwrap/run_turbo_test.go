// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

func TestRun_TurboCleanRunCompactsAndTaskOutputOrFailureStaysRaw(t *testing.T) {
	clean := auditFixture(t, "turbo_clean.txt")
	fakeBin(t, "turbo", "cat <<'EOF'\n"+clean+"EOF")
	r, out, _ := run(t, []string{"turbo", "run", "build"}, true)
	if r.ExitCode != 0 || !strings.Contains(out, "[x] turbo: 2 successful, 2 total (1 cached) in 4.213s") {
		t.Fatalf("exit=%d out=%q", r.ExitCode, out)
	}
	noisy := auditFixture(t, "turbo_output_line.txt")
	fakeBin(t, "turbo", "cat <<'EOF'\n"+noisy+"EOF")
	if _, out, _ := run(t, []string{"turbo", "run", "build"}, true); out != noisy {
		t.Fatalf("task output run changed: %q", out)
	}
	fakeBin(t, "turbo", "cat <<'EOF'\n"+clean+"EOF\nexit 1")
	if r, out, _ := run(t, []string{"turbo", "run", "build"}, true); r.ExitCode != 1 || out != clean {
		t.Fatalf("exit=%d changed=%v", r.ExitCode, out != clean)
	}
}
