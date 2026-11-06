// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

func TestRun_BazelWritesOnStderrAndStillCompacts(t *testing.T) {
	raw := auditFixture(t, "bazel_build_clean.txt")
	fakeBin(t, "bazel", "cat <<'EOF' >&2\n"+raw+"EOF")
	r, out, errOut := run(t, []string{"bazel", "build", "//app:main"}, true)
	if r.ExitCode != 0 || !strings.Contains(out+errOut, "[x] bazel: build completed successfully, 25 total actions in 12.345s") {
		t.Fatalf("exit=%d out=%q err=%q", r.ExitCode, out, errOut)
	}
}

func TestRun_BazelFailureAndWarningStayRaw(t *testing.T) {
	warn := auditFixture(t, "bazel_warn.txt")
	fakeBin(t, "bazel", "cat <<'EOF' >&2\n"+warn+"EOF")
	if _, out, errOut := run(t, []string{"bazel", "build", "//app:main"}, true); out+errOut != warn {
		t.Fatalf("warning run changed: %q %q", out, errOut)
	}
	raw := auditFixture(t, "bazel_build_clean.txt")
	fakeBin(t, "bazel", "cat <<'EOF' >&2\n"+raw+"EOF\nexit 1")
	r, out, errOut := run(t, []string{"bazel", "build", "//app:main"}, true)
	if r.ExitCode != 1 || out+errOut != raw {
		t.Fatalf("exit=%d changed=%v", r.ExitCode, out+errOut != raw)
	}
}
