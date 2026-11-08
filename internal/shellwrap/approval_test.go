// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// fakeShell is a RewriteAdapter whose rewrite output is the wrapped command.
type fakeShell struct{ command string }

func (f fakeShell) ShellCommand() (string, bool)             { return f.command, true }
func (f fakeShell) ShellRewrite(command string) (any, error) { return command, nil }
func (f fakeShell) ShellBlocked(string) bool                 { return false }

// rewritingHarness names one declared harness that rewrites shell commands
// and has exactly the given approval and unknown-rules capabilities, read
// from the capability table.
func rewritingHarness(t *testing.T, approves, unknownRules bool) string {
	t.Helper()
	for _, n := range core.AllHarnessCaps() {
		c := n.Caps
		if c.CanRewriteShellCommand && c.RewriteImpliesApproval == approves && c.UnknownUserRules == unknownRules {
			return n.Harness
		}
	}
	t.Fatalf("no rewriting harness with RewriteImpliesApproval=%v UnknownUserRules=%v", approves, unknownRules)
	return ""
}

func TestHandleRewriteLimitedToReadOnlyCommand(t *testing.T) {
	approving := rewritingHarness(t, true, true) // codex
	unknown := rewritingHarness(t, false, true)  // cursor
	plain := rewritingHarness(t, false, false)   // claude-code
	cases := []struct {
		name    string
		harness string
		command string
		rewrite bool
		skip    string
	}{
		{"chain rider on an approving harness", approving, "go version ; perl /tmp/x.pl", false, ReasonNotReadOnly},
		{"runner on an approving harness", approving, "make check", false, ReasonNotReadOnly},
		{"test runner on an approving harness", approving, "go test ./...", false, ReasonNotReadOnly},
		{"mutating gh on an approving harness", approving, "gh pr merge 12", false, ReasonNotReadOnly},
		{"read-only git on an approving harness", approving, "git status", true, ""},
		{"mutating gh where user rules are unknown", unknown, "gh repo delete x --yes", false, ReasonNotReadOnly},
		{"read-only git where user rules are unknown", unknown, "git status", true, ""},
		{"runner where the harness still asks", plain, "make check", true, ""},
		{"chain where the harness still asks", plain, "go version ; go vet ./...", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			r := Handle(c.harness, fakeShell{command: c.command}, "")
			if !r.IsShell {
				t.Fatal("IsShell = false")
			}
			if (r.Rewrite != nil) != c.rewrite {
				t.Fatalf("rewrite = %v, want %v (result %+v)", r.Rewrite, c.rewrite, r)
			}
			if r.Skip != c.skip {
				t.Fatalf("skip = %q, want %q", r.Skip, c.skip)
			}
		})
	}
}
