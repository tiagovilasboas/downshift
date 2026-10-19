// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

const exe = "/opt/ds/downshift"

func TestDecide(t *testing.T) {
	cases := []struct {
		name    string
		command string
		rewrite bool
		family  Family
		want    string // wrapped command when rewrite
		reason  string
	}{
		{"go test", "go test ./...", true, FamilyGoTest, "'/opt/ds/downshift' exec -- go test ./...", ReasonAllowlisted},
		{"go test flags", "go test -count=1 -run TestX ./internal/core", true, FamilyGoTest, "'/opt/ds/downshift' exec -- go test -count=1 -run TestX ./internal/core", ReasonAllowlisted},
		{"git status", "git status", true, FamilyGitStatus, "'/opt/ds/downshift' exec -- git status", ReasonAllowlisted},
		{"surrounding space", "  git status  ", true, FamilyGitStatus, "'/opt/ds/downshift' exec -- git status", ReasonAllowlisted},
		{"collapsed spaces", "go  test   ./...", true, FamilyGoTest, "'/opt/ds/downshift' exec -- go test ./...", ReasonAllowlisted},
		{"pytest bare", "pytest", true, FamilyPytest, "'/opt/ds/downshift' exec -- pytest", ReasonAllowlisted},
		{"pytest with args", "pytest -v tests/test_app.py", true, FamilyPytest, "'/opt/ds/downshift' exec -- pytest -v tests/test_app.py", ReasonAllowlisted},
		{"vitest bare", "vitest", true, FamilyVitest, "'/opt/ds/downshift' exec -- vitest", ReasonAllowlisted},
		{"vitest run", "vitest run", true, FamilyVitest, "'/opt/ds/downshift' exec -- vitest run", ReasonAllowlisted},
		{"npx vitest", "npx vitest", true, FamilyVitest, "'/opt/ds/downshift' exec -- npx vitest", ReasonAllowlisted},
		{"npx vitest run", "npx vitest run --reporter=verbose", true, FamilyVitest, "'/opt/ds/downshift' exec -- npx vitest run --reporter=verbose", ReasonAllowlisted},
		{"jest bare", "jest", true, FamilyJest, "'/opt/ds/downshift' exec -- jest", ReasonAllowlisted},
		{"jest run", "jest --runInBand", true, FamilyJest, "'/opt/ds/downshift' exec -- jest --runInBand", ReasonAllowlisted},
		{"npx jest", "npx jest", true, FamilyJest, "'/opt/ds/downshift' exec -- npx jest", ReasonAllowlisted},
		{"cypress run", "cypress run", true, FamilyCypress, "'/opt/ds/downshift' exec -- cypress run", ReasonAllowlisted},
		{"npx cypress run", "npx cypress run", true, FamilyCypress, "'/opt/ds/downshift' exec -- npx cypress run", ReasonAllowlisted},

		{"git log not allowlisted", "git log", false, "", "", ReasonNotAllowlisted},
		{"git diff", "git diff", false, "", "", ReasonNotAllowlisted},
		{"go build", "go build ./...", false, "", "", ReasonNotAllowlisted},
		{"bare go", "go", false, "", "", ReasonNotAllowlisted},
		{"path to git", "/usr/bin/git status", false, "", "", ReasonNotAllowlisted},
		{"git global option first", "git -C /tmp status", false, "", "", ReasonNotAllowlisted},
		{"sudo", "sudo go test ./...", false, "", "", ReasonNotAllowlisted},
		{"env assignment", "GOFLAGS=-v go test ./...", false, "", "", ReasonNotAllowlisted},
		{"empty", "", false, "", "", ReasonEmpty},
		{"blank", "   ", false, "", "", ReasonEmpty},

		{"already wrapped", "downshift exec -- go test ./...", false, "", "", ReasonAlreadyWrapped},
		{"wrapped twice", "downshift exec -- downshift exec -- git status", false, "", "", ReasonAlreadyWrapped},

		{"pipe", "go test ./... | tail", false, "", "", ReasonUnsafeChar},
		{"semicolon", "git status; rm -rf x", false, "", "", ReasonUnsafeChar},
		{"and list", "go test ./... && echo ok", false, "", "", ReasonUnsafeChar},
		{"background", "go test ./... &", false, "", "", ReasonUnsafeChar},
		{"redirect out", "go test ./... > out.txt", false, "", "", ReasonUnsafeChar},
		{"redirect in", "git status < x", false, "", "", ReasonUnsafeChar},
		{"dollar", "go test $PKG", false, "", "", ReasonUnsafeChar},
		{"backtick", "go test `pwd`", false, "", "", ReasonUnsafeChar},
		{"subshell", "go test $(pwd)", false, "", "", ReasonUnsafeChar},
		{"parens", "(git status)", false, "", "", ReasonUnsafeChar},
		{"newline", "git status\nrm -rf x", false, "", "", ReasonUnsafeChar},
		{"carriage return", "git status\r", false, "", "", ReasonUnsafeChar},
		{"trailing newline", "git status\n", false, "", "", ReasonUnsafeChar},
		{"tab", "go\ttest ./...", false, "", "", ReasonUnsafeChar},
		{"double quote", "go test -run \"TestX\" ./...", false, "", "", ReasonUnsafeChar},
		{"single quote", "go test -run 'TestX' ./...", false, "", "", ReasonUnsafeChar},
		{"glob star", "go test ./internal/*", false, "", "", ReasonUnsafeChar},
		{"glob question", "go test ./interna?", false, "", "", ReasonUnsafeChar},
		{"glob bracket", "go test ./[a-z]", false, "", "", ReasonUnsafeChar},
		{"brace", "go test ./{a,b}", false, "", "", ReasonUnsafeChar},
		{"tilde", "go test ~/x", false, "", "", ReasonUnsafeChar},
		{"backslash", "go test .\\x", false, "", "", ReasonUnsafeChar},
		{"comment", "git status #x", false, "", "", ReasonUnsafeChar},
		{"bang", "git status !1", false, "", "", ReasonUnsafeChar},
		{"caret", "go test -run ^TestX", false, "", "", ReasonUnsafeChar},
		{"non ascii", "go test ./ação", false, "", "", ReasonUnsafeChar},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, exe)
			if d.Rewrite != tc.rewrite || d.Family != tc.family || d.Reason != tc.reason {
				t.Fatalf("Decide(%q) = %+v, want rewrite=%v family=%q reason=%q", tc.command, d, tc.rewrite, tc.family, tc.reason)
			}
			if d.Command != tc.want {
				t.Fatalf("Command = %q, want %q", d.Command, tc.want)
			}
		})
	}
}

func TestDecide_TooLong(t *testing.T) {
	long := "go test " + string(make([]byte, 0))
	for len(long) <= maxCommandBytes {
		long += " ./x"
	}
	if d := Decide(long, exe); d.Rewrite || d.Reason != ReasonTooLong {
		t.Fatalf("Decide(long, exe) = %+v, want too_long", d)
	}
}

// A wrapped command must never be wrapped again: Decide on its own output
// always refuses.
func TestDecide_Idempotent(t *testing.T) {
	for _, c := range []string{"go test ./...", "git status"} {
		first := Decide(c, exe)
		if !first.Rewrite {
			t.Fatalf("Decide(%q) did not rewrite", c)
		}
		if second := Decide(first.Command, exe); second.Rewrite {
			t.Fatalf("Decide(%q) rewrote an already wrapped command: %+v", first.Command, second)
		}
	}
}

func TestDecide_BadExecutable(t *testing.T) {
	for _, bad := range []string{"", "downshift", "./downshift", "/opt/it's/downshift", "/opt/a\nb/downshift"} {
		if d := Decide("go test ./...", bad); d.Rewrite || d.Reason != ReasonBadExecutable {
			t.Fatalf("exe %q: %+v", bad, d)
		}
	}
	if q, ok := QuoteExecutable("/opt/my dir/$x/downshift"); !ok || q != "'/opt/my dir/$x/downshift'" {
		t.Fatalf("quote = %q %v", q, ok)
	}
}
