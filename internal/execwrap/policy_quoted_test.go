// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"testing"
)

const quotedExe = "'/opt/ds/downshift'"

func TestDecideQuotedAccepts(t *testing.T) {
	for _, tc := range []struct {
		name, command string
		family        Family
		want          string
	}{
		{"double-quoted grep pattern", `grep -rn "x y" .`, FamilyGrep,
			quotedExe + ` compact -- 'grep' '-rn' 'x y' '.'`},
		{"single-quoted find glob", `find . -name '*.go'`, FamilyFind,
			quotedExe + ` compact -- 'find' '.' '-name' '*.go'`},
		{"commit message is the git commit family", `git commit -m "fix: it's done"`, FamilyGitCommit,
			quotedExe + ` compact -- 'git' 'commit' '-m' 'fix: it'\''s done'`},
		{"glob inside double quotes", `find . -name "*.go"`, FamilyFind,
			quotedExe + ` compact -- 'find' '.' '-name' '*.go'`},
		{"single quote keeps $ and backtick literal", "grep -rn '$HOME `x` \\n !' .", FamilyGrep,
			quotedExe + " compact -- 'grep' '-rn' '$HOME `x` \\n !' '.'"},
		{"adjacent quotes join one word", `go test -run="Test"'A|B' ./...`, FamilyGoTest,
			quotedExe + ` compact -- 'go' 'test' '-run=TestA|B' './...'`},
		{"empty quoted word", `git commit -m ""`, FamilyGitCommit,
			quotedExe + ` compact -- 'git' 'commit' '-m' ''`},
		{"trailing 2>/dev/null", `go test ./... 2>/dev/null`, FamilyGoTest,
			quotedExe + ` compact --discard-stderr -- 'go' 'test' './...'`},
		{"trailing || true", `git status || true`, FamilyGitStatus,
			quotedExe + ` compact --exit-zero -- 'git' 'status'`},
		{"both trailing forms", `grep -rn "a" . 2>/dev/null || true`, FamilyGrep,
			quotedExe + ` compact --discard-stderr --exit-zero -- 'grep' '-rn' 'a' '.'`},
		{"plain assignment stays bare", `CI=1 go test -run 'X' ./...`, FamilyGoTest,
			`CI=1 ` + quotedExe + ` compact -- 'go' 'test' '-run' 'X' './...'`},
		{"utf-8 inside quotes", `git commit -m "café ✓"`, FamilyGitCommit,
			quotedExe + ` compact -- 'git' 'commit' '-m' 'café ✓'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := Decide(tc.command, "/opt/ds/downshift")
			if !d.Rewrite || d.Family != tc.family || d.Command != tc.want || d.Reason != ReasonAllowlistedQuoted {
				t.Fatalf("Decide(%q) = %+v\nwant command %s family %s", tc.command, d, tc.want, tc.family)
			}
		})
	}
}

func TestDecideQuotedRefuses(t *testing.T) {
	for _, command := range []string{
		`grep -rn "$HOME" .`, "grep -rn \"`id`\" .", `grep -rn "a\b" .`, `grep -rn "hi!" .`,
		`grep -rn "x .`, `grep -rn 'x .`, `grep -rn "x" . ; rm -rf /`, `grep -rn "x" . && rm x`,
		`grep -rn "x" . | wc -l`, `grep -rn "x" . &`, `grep -rn "x" $(pwd)`, "grep -rn \"x\" `pwd`",
		`grep -rn "x" . > out`, `grep -rn "x" . 2> err`, `grep -rn "x" . 2>/dev/null2`,
		`grep -rn "x" . 1>/dev/null`, `grep -rn "x" . &>/dev/null`, `grep -rn "x" * `,
		`grep -rn "x" ~`, "grep -rn 'a\nb' .", "grep -rn \"a\x00b\" .", "grep -rn 'a\x01' .",
		"grep -rn '\xff' .", `go test ./... || true 2>/dev/null`, `go test ./... || false`,
		`go test ./...||true`, `go test ./... 2>&1 || true`, `grep -rn " || true`,
		`grep -rn "a 2>/dev/null`, `'FOO=1' go test ./...`, `FOO="a b" go test ./...`,
		`PATH=/x go test -run 'X'`, `MYSQL_PWD=x mysql -e "SELECT 1"`, `git -c "core.pager=x" log`,
		`find . -name "x" "-delete"`, `downshift compact -- go test -run 'X'`,
		`echo "x"`, `cd "x" && go test`,
	} {
		if d := Decide(command, "/opt/ds/downshift"); d.Rewrite {
			t.Errorf("Decide(%q) rewrote: %+v", command, d)
		}
	}
}

// Every byte outside quotes stays in the safe set, so a metacharacter the
// quoted path does not name keeps its unsafe_character answer.
func TestDecideQuotedKeepsUnsafeReason(t *testing.T) {
	for _, command := range []string{`grep -rn "x" . | wc -l`, `go test -run 'X' ; ls`, `grep "$X" .`} {
		if d := Decide(command, "/opt/ds/downshift"); d.Reason != ReasonUnsafeChar {
			t.Errorf("Decide(%q).Reason = %q, want %q", command, d.Reason, ReasonUnsafeChar)
		}
	}
}

// A deny or ask rule is matched against the command as typed (blocked(command)
// in shellwrap) and against Argv and Segments: the argv as written, and the
// argv unquoted.
func TestDecideQuotedPermissionTexts(t *testing.T) {
	d := Decide(`CI=1 git commit -m "a b" 2>/dev/null`, "/opt/ds/downshift")
	if !d.Rewrite || d.Argv != `git commit -m "a b"` || !reflect.DeepEqual(d.Segments, []string{"git commit -m a b"}) {
		t.Fatalf("decision = %+v", d)
	}
}

// The approval path of Codex and Cursor stays exactly as strict: no quote,
// no 2>/dev/null and no || true is read-only, even around a read-only argv.
func TestReadOnlyStaysStrictForQuotedForms(t *testing.T) {
	for _, command := range []string{
		`git log --grep "fix"`, `git log --grep 'fix'`, `git status 2>/dev/null`, `git status || true`,
		`git diff 2>/dev/null || true`, `gh pr view "12"`, `ls 'dir'`, `git "status"`,
	} {
		if ReadOnly(command) {
			t.Errorf("ReadOnly(%q) = true, want false", command)
		}
		if d := Decide(command, "/opt/ds/downshift"); !d.Rewrite {
			t.Errorf("Decide(%q) should still wrap it for the compact rewrite: %+v", command, d)
		}
	}
	if !ReadOnly("git status") {
		t.Fatal("plain git status must stay read-only")
	}
}

// The session tag lands after the quoted binary only, never inside a word
// that holds the same text.
func TestWithSessionQuotedRewrite(t *testing.T) {
	d := Decide(`git commit -m "x' compact -- y" 2>/dev/null`, "/opt/ds/downshift")
	got := WithSession(d.Command, "abc123")
	want := quotedExe + ` compact --session=abc123 --discard-stderr -- 'git' 'commit' '-m' 'x'\'' compact -- y'`
	if got != want {
		t.Fatalf("WithSession =\n%s\nwant\n%s", got, want)
	}
	if plain := WithSession(quotedExe+" compact -- go test", "abc123"); plain != quotedExe+" compact --session=abc123 -- go test" {
		t.Fatalf("plain rewrite tag = %s", plain)
	}
}

// The metachar class of a skip names what is still refused, not a quote the
// policy now accepts.
func TestMetacharClassAfterQuotes(t *testing.T) {
	for _, tc := range []struct{ command, want string }{
		{`grep -rn "x" . | wc -l`, MetaPipe},
		{`git commit -m "a;b" ; ls`, MetaSemicolon},
		{`grep "$X" .`, MetaSubst},
		{"grep \"`x`\" .", MetaBacktick},
		{`grep "a\b" .`, MetaBackslash},
		{`grep "x .`, MetaQuote},
		{`go test -run 'X' > out`, MetaRedirect},
		{`go test ./... 2>/dev/null`, ""},
		{`go test -run "X" || true`, ""},
		{`go test ./... || false`, MetaOr},
	} {
		if got := MetacharClass(tc.command); got != tc.want {
			t.Errorf("MetacharClass(%q) = %q, want %q", tc.command, got, tc.want)
		}
	}
	if c, f := SkipDetail(SkipMetachar, `go test -run "A B" | tail`); c != MetaPipe || f != string(FamilyGoTest) {
		t.Fatalf("SkipDetail = %q, %q", c, f)
	}
	if r := SkipReason(Decision{Reason: ReasonNotAllowlisted}, `echo "x"`); r != SkipNotAllowlisted {
		t.Fatalf("SkipReason(echo quoted) = %q", r)
	}
}

// Two forms zsh reads unlike sh stay off the quoted path: an unquoted word
// that starts with = (=cmd expansion) and a single-quoted piece right after
// another one (under rc_quotes two single quotes are one quote).
func TestQuotedLexerRefusesZshForms(t *testing.T) {
	for _, tc := range []struct {
		command string
		bad     int
	}{
		{`go test =foo 'x'`, 8},
		{`go test 'x' =foo`, 12},
		{`git commit -m 'it''s'`, 18},
		{`git log --grep 'a'''`, 18},
	} {
		if _, bad := lexQuoted(tc.command); bad != tc.bad {
			t.Errorf("lexQuoted(%q) bad = %d, want %d", tc.command, bad, tc.bad)
		}
		if d := Decide(tc.command, "/opt/ds/downshift"); d.Rewrite {
			t.Errorf("Decide(%q) wrapped it: %q", tc.command, d.Command)
		}
	}
	for _, command := range []string{`go test -run=X 'y'`, `go test A=b 'x'`, `git commit -m "a"'b'`, `git commit -m 'a'"b"'c'`, `git log '=x'`} {
		if d := Decide(command, "/opt/ds/downshift"); !d.Rewrite {
			t.Errorf("Decide(%q) = %+v, want a wrap", command, d)
		}
	}
}

// The session tag finds the binary in every tail-flag form: the flags follow
// ` compact --`, so the tag lands before them.
func TestWithSessionTailFlagForms(t *testing.T) {
	for command, flags := range map[string]string{
		`go test -run "X" 2>/dev/null`:         " " + FlagDiscardStderr,
		`go test -run "X" || true`:             " " + FlagExitZero,
		`go test -run "X" 2>/dev/null || true`: " " + FlagDiscardStderr + " " + FlagExitZero,
	} {
		got := WithSession(Decide(command, "/opt/ds/downshift").Command, "abc123")
		if want := quotedExe + " compact --session=abc123" + flags + " -- 'go' 'test' '-run' 'X'"; got != want {
			t.Errorf("WithSession(%q) =\n%s\nwant\n%s", command, got, want)
		}
	}
}
