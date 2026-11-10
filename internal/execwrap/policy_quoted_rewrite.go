// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// The rewritten form of a quoted command, and the session tag that must find
// the wrapper in it without reading inside a quoted word.

// Flags the quoted rewrite puts between `compact` and `--`, one per trailing
// form of the original command. The runner reads them (run_tail.go).
const (
	FlagDiscardStderr = "--discard-stderr"
	FlagExitZero      = "--exit-zero"
)

// QuoteWord single-quotes one argv word for a POSIX shell: a quote inside is
// closed, written as an escaped quote and reopened, so the shell sees exactly w.
func QuoteWord(w string) string {
	return "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
}

// rewriteQuoted builds `[assigns] 'exe' compact [flags] -- 'w0' 'w1' ...`.
// assigns are plain safe words, kept bare so the shell still applies them;
// every argv word is quoted, so no byte of it is a metacharacter.
func rewriteQuoted(assigns []shellWord, quotedExe string, tail shellTail, argv []string) string {
	var b strings.Builder
	for _, a := range assigns {
		b.WriteString(a.text + " ")
	}
	b.WriteString(quotedExe + " compact")
	if tail.discardStderr {
		b.WriteString(" " + FlagDiscardStderr)
	}
	if tail.exitZero {
		b.WriteString(" " + FlagExitZero)
	}
	b.WriteString(" --")
	for _, w := range argv {
		b.WriteString(" " + QuoteWord(w))
	}
	return b.String()
}

// tagSessions inserts --session=<tag> after every ` compact` that follows a
// closing quote: the quoted binary. A quote that opens (a quoted argv word,
// the reopening half of an escaped quote) is never a match, so text inside a word is not
// changed. Outside quotes a backslash escapes the next byte.
func tagSessions(command, tag string) string {
	return insertAfterQuotedCompact(command, " compact --session="+tag+" --")
}

// insertAfterQuotedCompact replaces the " compact --" that follows each quoted
// binary with with. The tail flags follow the binary as ` compact
// --discard-stderr`, which also starts with the mark, so with lands before them.
func insertAfterQuotedCompact(command, with string) string {
	const mark = " compact --"
	var b strings.Builder
	in := false
	for i := 0; i < len(command); i++ {
		c := command[i]
		b.WriteByte(c)
		switch {
		case c == '\\' && !in && i+1 < len(command):
			i++
			b.WriteByte(command[i])
		case c == '\'':
			in = !in
			if !in && strings.HasPrefix(command[i+1:], mark) {
				b.WriteString(with)
				i += len(mark)
			}
		}
	}
	return b.String()
}

// WithHarness names the harness on a wrapped command so the runner can log it
// even when the harness exports no session variable to the shell:
// `<exe> compact -- x` becomes `<exe> compact --harness=<name> -- x`, once per
// wrapped segment. A name that is not lowercase letters, digits or dashes, or
// a command already named, is returned unchanged.
func WithHarness(command, harness string) string {
	if harness == "" || strings.Contains(command, " compact --harness=") {
		return command
	}
	for _, c := range harness {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return command
		}
	}
	return insertAfterQuotedCompact(command, " compact --harness="+harness+" --")
}
