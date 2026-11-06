// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"strings"
	"testing"
)

func lockfileDiffText(extra string) string {
	pad := strings.Repeat(" "+`      "integrity": "sha512-abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGH",`+"\n", 30)
	return "diff --git a/package-lock.json b/package-lock.json\nindex 1a2b3c4..5d6e7f8 100644\n" +
		"--- a/package-lock.json\n+++ b/package-lock.json\n@@ -1,35 +1,35 @@\n" +
		"     \"node_modules/left\": {\n-      \"version\": \"1.0.0\",\n+      \"version\": \"1.1.0\",\n" + extra + pad
}

func TestRun_GitDiffOfLockfileIsSummarised(t *testing.T) {
	raw := lockfileDiffText("")
	fakeBin(t, "git", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"git", "diff"}, true)
	if r.ExitCode != 0 || !strings.Contains(out, "package-lock.json: 0 added, 0 removed, 1 changed") ||
		!strings.Contains(out, "~ left 1.0.0 -> 1.1.0") || !strings.Contains(out, "[downshift compact:") {
		t.Fatalf("exit=%d out=%.300q", r.ExitCode, out)
	}
}

func TestRun_GitDiffOfLockfileWithDeprecationStaysAChangeDiff(t *testing.T) {
	raw := lockfileDiffText("+      \"deprecated\": \"use right\",\n")
	fakeBin(t, "git", "cat <<'EOF'\n"+raw+"EOF")
	_, out, _ := run(t, []string{"git", "diff"}, true)
	if strings.Contains(out, "package-lock.json: 0 added") || !strings.Contains(out, `+      "deprecated": "use right",`) {
		t.Fatalf("summarised a diff with a deprecation notice: %.300q", out)
	}
}
