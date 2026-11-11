// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "testing"

func TestCompoundShape(t *testing.T) {
	for _, tc := range []struct{ cmd, inner, kind string }{
		{"go test ./... 2>&1 | tail -20", "go-test", KindRedirect},
		{"go test ./... | tail -20", "go-test", KindPipeOther},
		{"cd sub && go test ./...", "go-test", ""},
		{"git log -n 5 > out.txt", "git-log", KindRedirect},
		{"FOO=1 git status | grep x", "git-status", KindPipeGrep},
		{"echo $(git rev-parse HEAD)", "git-other", KindSubst},
		{"python3 run.py | head", "", KindPipeOther},
		{`grep -rn "a|b" . | wc -l`, "grep", KindPipeOther},
		{"downshift compact -- go test | tail", "", KindPipeOther},
		{"git status", "", ""},     // simple command: no shape
		{"python3 run.py", "", ""}, // simple, not allowlisted: no shape
	} {
		inner, kind := CompoundShape(tc.cmd)
		if inner != tc.inner || (tc.kind != "" && kind != tc.kind) {
			t.Errorf("%q: (%q, %q), want (%q, %q)", tc.cmd, inner, kind, tc.inner, tc.kind)
		}
		if simple := tc.cmd == "git status" || tc.cmd == "python3 run.py"; simple != (kind == "") {
			t.Errorf("%q: kind %q, want empty only for a simple command", tc.cmd, kind)
		}
	}
}

func TestInnerFamily_QuotesDoNotSplit(t *testing.T) {
	if f := InnerFamily(`echo "x | go test ./..."`); f != FamilyNone {
		t.Errorf("a pipe inside quotes split the command: %q", f)
	}
	if f := InnerFamily(`echo "unclosed | go test`); f != FamilyNone {
		t.Errorf("an unclosed quote split the command: %q", f)
	}
}
