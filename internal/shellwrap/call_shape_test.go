// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package shellwrap

import "testing"

// shapedShell is a RewriteAdapter that also reports the call's background flag
// and whether the harness will prompt the user.
type shapedShell struct {
	fakeShell
	background bool
	unprompted bool
}

func (s shapedShell) ShellRunsInBackground() bool  { return s.background }
func (s shapedShell) ShellPromptsSuppressed() bool { return s.unprompted }

// shapedAdvice is an AdviseAdapter with the same two reports.
type shapedAdvice struct {
	command    string
	background bool
	unprompted bool
}

func (s shapedAdvice) ShellCommand() (string, bool)  { return s.command, true }
func (s shapedAdvice) ShellBlocked(string) bool      { return false }
func (s shapedAdvice) AdviseMessage(w string) string { return w }
func (s shapedAdvice) ShellRunsInBackground() bool   { return s.background }
func (s shapedAdvice) ShellPromptsSuppressed() bool  { return s.unprompted }

// A harness that runs calls without asking (bypassPermissions, dontAsk) has
// only its deny rules, which may not match the rewritten command, so only one
// read-only command is wrapped. A background call is never wrapped: the
// compactor holds output until exit.
func TestHandleRewriteHonoursCallShape(t *testing.T) {
	plain := rewritingHarness(t, false, false) // claude-code
	cases := []struct {
		name       string
		command    string
		background bool
		unprompted bool
		rewrite    bool
		skip       string
	}{
		{"unprompted mutating gh", "gh pr merge 12 --squash", false, true, false, ReasonNotReadOnly},
		{"unprompted test runner", "go test ./...", false, true, false, ReasonNotReadOnly},
		{"unprompted chain", "go version ; go vet ./...", false, true, false, ReasonNotReadOnly},
		{"unprompted read-only git", "git status", false, true, true, ""},
		{"prompting test runner", "go test ./...", false, false, true, ""},
		{"prompting mutating gh still reaches the prompt wrapped", "gh pr merge 12 --squash", false, false, true, ""},
		{"background test runner", "go test ./...", true, false, false, ReasonBackground},
		{"background read-only git", "git status", true, false, false, ReasonBackground},
		{"background and unprompted", "go test ./...", true, true, false, ReasonBackground},
		{"background unallowlisted keeps its own reason", "vim x", true, false, false, "not_allowlisted"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			ev := shapedShell{fakeShell: fakeShell{command: c.command}, background: c.background, unprompted: c.unprompted}
			r := Handle(plain, ev, "")
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

// The advise path applies the same two checks: the model would retry the
// wrapped form in the foreground, or where nothing prompts for it.
func TestHandleAdviseHonoursCallShape(t *testing.T) {
	cases := []struct {
		name       string
		command    string
		background bool
		unprompted bool
		advise     bool
		skip       string
	}{
		{"plain test runner", "go test ./...", false, false, true, ""},
		{"background test runner", "go test ./...", true, false, false, ReasonBackground},
		{"unprompted test runner", "go test ./...", false, true, false, ReasonNotReadOnly},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withEnv(t, true, nil, true)
			r := Handle("kirocrew", shapedAdvice{command: c.command, background: c.background, unprompted: c.unprompted}, "")
			if (r.Advise != "") != c.advise || r.Skip != c.skip {
				t.Fatalf("advise = %q skip = %q, want advise=%v skip=%q", r.Advise, r.Skip, c.advise, c.skip)
			}
		})
	}
}
