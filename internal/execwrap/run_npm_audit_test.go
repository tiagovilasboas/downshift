// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func auditFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "compressor", "testdata", "corpus", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRun_NpmAuditBelowLevelIsSummarised(t *testing.T) {
	raw := auditFixture(t, "npm_audit_clean.txt")
	fakeBin(t, "npm", "cat <<'EOF'\n"+raw+"EOF")
	r, out, _ := run(t, []string{"npm", "audit"}, true)
	if r.ExitCode != 0 || !strings.Contains(out, "2 moderate (advisories cut)") ||
		!strings.Contains(out, "Command Injection in lodash - https://github.com/advisories/GHSA-35jh-r3h4-6jhm") {
		t.Fatalf("exit=%d out=%.300q", r.ExitCode, out)
	}
}

func TestRun_NpmAuditWithFindingsAndNonZeroExitStaysRaw(t *testing.T) {
	raw := auditFixture(t, "npm_audit_clean.txt")
	fakeBin(t, "npm", "cat <<'EOF'\n"+raw+"EOF\nexit 1")
	r, out, _ := run(t, []string{"npm", "audit"}, true)
	if r.ExitCode != 1 || out != raw {
		t.Fatalf("exit=%d raw=%v", r.ExitCode, out == raw)
	}
}

func TestRun_NpmInstallKeepsDeprecationRawAndSummarisesCleanRun(t *testing.T) {
	warn := "npm warn deprecated inflight@1.0.6: not supported\nadded 120 packages, and audited 121 packages in 4s\n\nfound 0 vulnerabilities\n"
	fakeBin(t, "npm", "cat <<'EOF'\n"+warn+"EOF")
	if _, out, _ := run(t, []string{"npm", "install"}, true); out != warn {
		t.Fatalf("deprecation run changed: %q", out)
	}
	clean := "added 120 packages, and audited 121 packages in 4s\n\n30 packages are looking for funding\n  run `npm fund` for details\n\nfound 0 vulnerabilities\n"
	fakeBin(t, "npm", "cat <<'EOF'\n"+clean+"EOF")
	_, out, _ := run(t, []string{"npm", "install"}, true)
	if !strings.Contains(out, "[x] npm: added 120 packages in 4s") {
		t.Fatalf("clean run not summarised: %q", out)
	}
}

func TestRun_GhIssueViewMarkdownIsSlimmedAndErrorTextStaysRaw(t *testing.T) {
	clean := auditFixture(t, "gh_issue_view_clean.txt")
	fakeBin(t, "gh", "cat <<'EOF'\n"+clean+"EOF")
	r, out, _ := run(t, []string{"gh", "issue", "view", "812"}, true)
	if r.ExitCode != 0 || strings.Contains(out, "img.shields.io") || !strings.Contains(out, "[roadmap](docs/roadmap.md)") {
		t.Fatalf("exit=%d out=%.300q", r.ExitCode, out)
	}
	raw := auditFixture(t, "gh_issue_view_error_text.txt")
	fakeBin(t, "gh", "cat <<'EOF'\n"+raw+"EOF")
	_, out, _ = run(t, []string{"gh", "issue", "view", "812"}, true)
	if out != raw {
		t.Fatalf("prose with an error word was changed: %.200q", out)
	}
}
