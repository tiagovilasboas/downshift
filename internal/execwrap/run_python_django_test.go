// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

// The fixtures are hand-written; the fake binary replays them verbatim.
func TestRun_DjangoCleanCompactsWarningAndFailureStayRaw(t *testing.T) {
	argv := []string{"django-admin", "test"}
	clean := auditFixture(t, "django_test_many_ok_clean.txt")
	fakeBin(t, "django-admin", "cat <<'EOF'\n"+clean+"EOF")
	r, out, _ := run(t, argv, true)
	if r.ExitCode != 0 || len(out) >= len(clean) || !strings.Contains(out, "lines elided]") {
		t.Fatalf("clean: exit=%d len=%d/%d", r.ExitCode, len(out), len(clean))
	}
	warn := auditFixture(t, "django_check_warning_preserve.txt")
	fakeBin(t, "django-admin", "cat <<'EOF'\n"+warn+"EOF")
	if _, out, _ := run(t, argv, true); out != warn {
		t.Fatalf("warning run changed: %q", out)
	}
	fakeBin(t, "django-admin", "cat <<'EOF'\n"+clean+"EOF\nexit 1")
	if r, out, _ := run(t, argv, true); r.ExitCode != 1 || out != clean {
		t.Fatalf("failed run: exit=%d changed=%v", r.ExitCode, out != clean)
	}
}
