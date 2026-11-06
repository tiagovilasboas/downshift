// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// posixWords splits a rewritten command the way a POSIX shell does for the
// forms the rewrite emits: bare words, '...' and a backslash outside quotes.
// unquotedMeta is the first byte outside quotes that is neither a space, a
// safe byte nor an escaped quote; it is 0 when there is none.
func posixWords(s string) (words []string, unquotedMeta byte, ok bool) {
	var cur strings.Builder
	in, started := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case in && c == '\'':
			in = false
		case in:
			cur.WriteByte(c)
		case c == '\'':
			in, started = true, true
		case c == '\\' && i+1 < len(s) && s[i+1] == '\'':
			cur.WriteByte('\'')
			started = true
			i++
		case c == ' ':
			if started {
				words = append(words, cur.String())
			}
			cur.Reset()
			started = false
		case safeByte(c):
			cur.WriteByte(c)
			started = true
		default:
			if unquotedMeta == 0 {
				unquotedMeta = c
			}
			cur.WriteByte(c)
			started = true
		}
	}
	if in {
		return nil, 0, false
	}
	if started {
		words = append(words, cur.String())
	}
	return words, unquotedMeta, true
}

// allowedWord reports whether w is in the alphabet a quoted word may carry:
// valid UTF-8 with no control byte.
func allowedWord(w string) bool {
	if !utf8.ValidString(w) {
		return false
	}
	for i := 0; i < len(w); i++ {
		if w[i] < 0x20 || w[i] == 0x7f {
			return false
		}
	}
	return true
}

// checkQuotedDecision holds a quoted Decide answer to the lexer: same family,
// and the rewrite splits into the plain assignments, the wrapper and exactly
// the lexed argv. A wrapped command is never wrapped again.
func checkQuotedDecision(t *testing.T, trimmed string, d Decision) {
	t.Helper()
	core, tail := cutShellTail(trimmed)
	words, bad := lexQuoted(core)
	n, at := plainAssignments(words)
	if bad >= 0 || at >= 0 {
		t.Fatalf("quoted rewrite of %q the lexer refuses at %d/%d", trimmed, bad, at)
	}
	var argv []string
	for _, w := range words[n:] {
		argv = append(argv, w.text)
	}
	if FamilyOf(argv) != d.Family || d.Family == FamilyNone {
		t.Fatalf("family %q, lexed argv %q has %q", d.Family, argv, FamilyOf(argv))
	}
	checkRewrite(t, d.Command, n, argv, tail)
	if again := Decide(d.Command, "/opt/ds/downshift"); again.Rewrite {
		t.Fatalf("wrapped command was wrapped again: %q", d.Command)
	}
}

// checkRoundTrip asserts parse(rewrite(argv)) == argv with no unquoted
// metacharacter in the rewritten string.
func checkRoundTrip(t *testing.T, argv []string, tail shellTail) {
	t.Helper()
	checkRewrite(t, rewriteQuoted(nil, quotedExe, tail, argv), 0, argv, tail)
}

// checkRewrite splits cmd like a POSIX shell and asserts it is skip plain
// assignment words, the wrapper with the tail flags, then exactly argv.
func checkRewrite(t *testing.T, cmd string, skip int, argv []string, tail shellTail) {
	t.Helper()
	got, meta, ok := posixWords(cmd)
	if !ok || meta != 0 || len(got) < skip {
		t.Fatalf("rewrite %q: ok=%v unquoted metachar %q", cmd, ok, meta)
	}
	got = got[skip:]
	head := []string{"/opt/ds/downshift", "compact"}
	if tail.discardStderr {
		head = append(head, FlagDiscardStderr)
	}
	if tail.exitZero {
		head = append(head, FlagExitZero)
	}
	head = append(head, "--")
	if len(got) < len(head) || !reflect.DeepEqual(got[:len(head)], head) || !reflect.DeepEqual(got[len(head):], argv) {
		t.Fatalf("parse(rewrite(%q)) = %q", argv, got)
	}
	if tagged := WithSession(cmd, "abc123"); tagged != strings.Replace(cmd, " compact --", " compact --session=abc123 --", 1) {
		t.Fatalf("session tag touched a word: %q", tagged)
	}
}

// FuzzQuotedWords checks two properties. A command the quoted lexer accepts
// rewrites to a string a POSIX shell splits back into the same argv. And any
// words from the allowed alphabet (the fuzz input split on NUL) survive the
// rewrite unchanged, with no unquoted metacharacter.
func FuzzQuotedWords(f *testing.F) {
	for _, s := range []string{
		`grep -rn "x y" .`, `find . -name '*.go'`, `git commit -m "it's"`, `a"b"'c'`,
		`x 'a'"'"'b'`, "w\x00it's\x00$(id)\x00`x`\x00 \x00", `"" ''`, "go test ./... || true",
		"grep 'a' 2>/dev/null", `x "a' compact -- b"`, "café\x00✓",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		core, tail := cutShellTail(strings.Trim(s, " "))
		if words, bad := lexQuoted(core); bad < 0 && len(words) > 0 {
			argv := make([]string, len(words))
			for i, w := range words {
				argv[i] = w.text
			}
			checkRoundTrip(t, argv, tail)
		}
		var argv []string
		for _, w := range strings.Split(s, "\x00") {
			if allowedWord(w) {
				argv = append(argv, w)
			}
		}
		if len(argv) > 0 {
			checkRoundTrip(t, argv, shellTail{})
		}
	})
}
