// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

func TestRun_NxCleanRunCompactsAndTaskOutputOrFailureStaysRaw(t *testing.T) {
	clean := auditFixture(t, "nx_clean.txt")
	fakeBin(t, "nx", "cat <<'EOF'\n"+clean+"EOF")
	r, out, _ := run(t, []string{"nx", "run", "myapp:build"}, true)
	if r.ExitCode != 0 || !strings.Contains(out, "[x] nx: ran target build for project myapp in 2s") {
		t.Fatalf("exit=%d out=%q", r.ExitCode, out)
	}
	noisy := auditFixture(t, "nx_output_line.txt")
	fakeBin(t, "nx", "cat <<'EOF'\n"+noisy+"EOF")
	if _, out, _ := run(t, []string{"nx", "run", "myapp:build"}, true); out != noisy {
		t.Fatalf("task output run changed: %q", out)
	}
	fakeBin(t, "nx", "cat <<'EOF'\n"+clean+"EOF\nexit 1")
	if r, out, _ := run(t, []string{"nx", "run", "myapp:build"}, true); r.ExitCode != 1 || out != clean {
		t.Fatalf("exit=%d changed=%v", r.ExitCode, out != clean)
	}
}
