// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"math/rand"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// shellSplit asks /bin/sh to split words the way it would run them.
func shellSplit(t *testing.T, words string) []string {
	t.Helper()
	out, err := exec.Command("/bin/sh", "-c", `printf '%s\0' `+words).Output()
	if err != nil {
		t.Fatalf("sh -c %q: %v", words, err)
	}
	got := strings.Split(string(out), "\x00")
	return got[:len(got)-1]
}

// randomChunk is one piece of a word: bare safe bytes, a single-quoted run
// or a double-quoted run, each from the alphabet the quoted path accepts.
func randomChunk(r *rand.Rand) string {
	const safe = "abcXYZ019-_./:=,+@%"
	const single = safe + " \"$`\\!*?;|&<>()#~{}[]é✓"
	const double = safe + " '*?;|&<>()#~{}[]é✓"
	pick := func(set string, n int) string {
		runes := []rune(set)
		var b strings.Builder
		for i := 0; i < n; i++ {
			b.WriteRune(runes[r.Intn(len(runes))])
		}
		return b.String()
	}
	switch r.Intn(3) {
	case 0:
		return pick(safe, 1+r.Intn(5))
	case 1:
		return "'" + pick(single, r.Intn(6)) + "'"
	}
	return `"` + pick(double, r.Intn(6)) + `"`
}

// The lexer splits every accepted command exactly as /bin/sh does, and the
// rewrite of its argv splits back to the same argv in /bin/sh.
func TestQuotedLexAndRewriteMatchShell(t *testing.T) {
	r := rand.New(rand.NewSource(20261106))
	commands := []string{`grep -rn "x y" .`, `find . -name '*.go'`, `git commit -m "it's ok"`, `a"b"'c'd`, `"" ''`}
	for i := 0; i < 300; i++ {
		var words []string
		for w := 0; w < 1+r.Intn(4); w++ {
			var b strings.Builder
			for c := 0; c < 1+r.Intn(3); c++ {
				b.WriteString(randomChunk(r))
			}
			words = append(words, b.String())
		}
		commands = append(commands, strings.Join(words, " "))
	}
	for _, cmd := range commands {
		lexed, bad := lexQuoted(cmd)
		if bad >= 0 && zshRefusal(cmd, bad) {
			continue // a form zsh reads differently: refused on purpose
		}
		if bad >= 0 {
			t.Fatalf("lexQuoted(%q) refused at %d", cmd, bad)
		}
		argv := make([]string, len(lexed))
		quoted := make([]string, len(lexed))
		for i, w := range lexed {
			argv[i], quoted[i] = w.text, QuoteWord(w.text)
		}
		if sh := shellSplit(t, cmd); !reflect.DeepEqual(sh, argv) {
			t.Fatalf("sh split %q as %q, lexer as %q", cmd, sh, argv)
		}
		if sh := shellSplit(t, strings.Join(quoted, " ")); !reflect.DeepEqual(sh, argv) {
			t.Fatalf("sh split the rewrite of %q as %q", argv, sh)
		}
	}
}

// zshRefusal reports whether the lexer refused cmd at bad for one of the two
// forms zsh reads differently: an unquoted word that starts with =, or a
// single quote right after a closing one.
func zshRefusal(cmd string, bad int) bool {
	atWordStart := bad == 0 || cmd[bad-1] == ' '
	return (cmd[bad] == '=' && atWordStart) || (cmd[bad] == '\'' && bad > 0 && cmd[bad-1] == '\'')
}
