// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"strings"
	"unicode/utf8"
)

// Quoted words: one command whose words may be quoted, as agents write
// `grep -rn "x" .`, `find . -name '*.go'` or `git commit -m "msg"`, with an
// optional exact `2>/dev/null` and an optional exact `|| true` at the end.
//
//	'...'  any byte but the quote, a control byte or invalid UTF-8
//	"..."  the same, and no $, backtick, \ or ! (expansion, escape, history)
//
// zsh reads two forms differently, so both are refused: an unquoted word
// that starts with = (zsh expands =cmd to a path) and a single-quoted piece
// right after another one ('it''s is it's under rc_quotes).
//
// Every byte outside quotes still passes the safe-byte gate, so no list,
// pipe, background, redirect, substitution, glob, tilde or comment can
// appear. The policy builds argv itself, exactly as the shell would, and the
// rewrite single-quotes every word (policy_quoted_rewrite.go), so the
// program receives the same argv with or without the wrapper. This path is
// only for the compact rewrite: ReadOnly keeps the strict safe-byte gate.
// Chains and compound shapes use it only for the command they wrap, never
// for a part they keep as written (decideQuotedPart, policy_compound.go).
// Threat note: docs/design/compactor-default-on-adr.md.

// ReasonAllowlistedQuoted is the reason for a wrapped quoted command.
const ReasonAllowlistedQuoted = "allowlisted_quoted"

// The two trailing forms the quoted path accepts, each exactly as written.
const (
	devNullTail = " 2>/dev/null"
	orTrueTail  = " || true"
)

// shellWord is one word of a quoted command.
type shellWord struct {
	text  string // the word the program receives
	raw   string // the word as written, quotes included
	plain bool   // written with no quote
	start int    // index of the word in the command
}

// decideQuoted wraps one allowlisted command with quoted words or one of the
// trailing forms, or refuses it as unsafe_character.
func decideQuoted(trimmed, exe string) Decision {
	if len(trimmed) > maxCommandBytes {
		return Decision{Reason: ReasonTooLong}
	}
	core, tail := cutShellTail(trimmed)
	words, bad := lexQuoted(core)
	if bad >= 0 || len(words) == 0 {
		return Decision{Reason: ReasonUnsafeChar}
	}
	n, at := plainAssignments(words)
	if at >= 0 {
		return Decision{Reason: ReasonUnsafeChar}
	}
	argv, raw := make([]string, 0, len(words)-n), make([]string, 0, len(words)-n)
	for _, w := range words[n:] {
		argv, raw = append(argv, w.text), append(raw, w.raw)
	}
	if len(argv) == 0 {
		return Decision{Reason: ReasonNotAllowlisted}
	}
	if argv[0] == "downshift" {
		return Decision{Reason: ReasonAlreadyWrapped}
	}
	family := FamilyOf(argv)
	if family == FamilyNone || (n > 0 && isDBClient(argv[0])) {
		return Decision{Reason: ReasonNotAllowlisted}
	}
	quoted, ok := QuoteExecutable(exe)
	if !ok {
		return Decision{Family: family, Reason: ReasonBadExecutable}
	}
	d := Decision{
		Rewrite: true,
		Command: rewriteQuoted(words[:n], quoted, tail, argv),
		Family:  family,
		Argv:    strings.Join(raw, " "),
		Reason:  ReasonAllowlistedQuoted,
	}
	// A deny or ask rule may be written either way: as typed, or unquoted.
	if unquoted := strings.Join(argv, " "); unquoted != d.Argv {
		d.Segments = []string{unquoted}
	}
	return d
}

// shellTail is what the trailing forms asked for.
type shellTail struct {
	discardStderr bool // a trailing 2>/dev/null
	exitZero      bool // a trailing || true
}

// cutShellTail splits off a trailing ` || true`, then a trailing
// ` 2>/dev/null`, in that order only. Text cut from inside a quote leaves
// that quote unclosed, so the lexer then refuses the command.
func cutShellTail(command string) (string, shellTail) {
	var t shellTail
	if strings.HasSuffix(command, orTrueTail) {
		t.exitZero = true
		command = strings.TrimRight(strings.TrimSuffix(command, orTrueTail), " ")
	}
	if strings.HasSuffix(command, devNullTail) {
		t.discardStderr = true
		command = strings.TrimRight(strings.TrimSuffix(command, devNullTail), " ")
	}
	return command, t
}

// lexQuoted splits command into words the way a POSIX shell would for the
// accepted alphabet. bad is the index of the first refused byte (for an
// unclosed quote, the opening quote), and -1 when the whole command is
// accepted.
func lexQuoted(command string) (words []shellWord, bad int) {
	var text strings.Builder
	start, plain, singleEnd := -1, true, -2
	flush := func(end int) {
		if start >= 0 {
			words = append(words, shellWord{text: text.String(), raw: command[start:end], plain: plain, start: start})
		}
		text.Reset()
		start, plain = -1, true
	}
	for i := 0; i < len(command); {
		c := command[i]
		switch {
		case c == ' ':
			flush(i)
			i++
			continue
		case c == '\'' && i == singleEnd+1:
			return nil, i // zsh rc_quotes: '' inside single quotes is a quote
		case c == '\'' || c == '"':
			end, at := closeQuote(command, i)
			if at >= 0 {
				return nil, at
			}
			if c == '\'' {
				singleEnd = end
			}
			if start < 0 {
				start = i
			}
			text.WriteString(command[i+1 : end])
			plain, i = false, end+1
			continue
		case !safeByte(c), c == '=' && start < 0:
			return nil, i // an unquoted leading = is zsh's =cmd expansion
		}
		if start < 0 {
			start = i
		}
		text.WriteByte(c)
		i++
	}
	flush(len(command))
	return words, -1
}

// closeQuote returns the index of the quote that closes the one at open, or
// the index of the first byte the quote may not hold (open itself when the
// quote never closes).
func closeQuote(command string, open int) (end, bad int) {
	q := command[open]
	for i := open + 1; i < len(command); i++ {
		c := command[i]
		switch {
		case c == q:
			return i, -1
		case c < 0x20 || c == 0x7f:
			return 0, i
		case q == '"' && (c == '$' || c == '`' || c == '\\' || c == '!'):
			return 0, i
		case c >= utf8.RuneSelf:
			r, size := utf8.DecodeRuneInString(command[i:])
			if r == utf8.RuneError && size == 1 {
				return 0, i
			}
			i += size - 1
		}
	}
	return 0, open
}

// plainAssignments counts the leading NAME=value words written with no
// quote. A quoted word ends them: the shell runs `'A=1' go` as a program
// named A=1. A word that opens as NAME= with a quoted value (A="x y") is an
// assignment the quoted path does not take; at is the index of its first
// quote, and -1 when there is none.
func plainAssignments(words []shellWord) (n, at int) {
	for n < len(words) && words[n].plain && isEnvAssignment(words[n].text) {
		n++
	}
	if n < len(words) && !words[n].plain {
		if name, _, found := strings.Cut(words[n].raw, "="); found && validEnvName(name) {
			return n, words[n].start + strings.IndexAny(words[n].raw, `'"`)
		}
	}
	return n, -1
}

// firstRefused is the index of the first byte that keeps command off the
// quoted path, or -1 when it lexes: the byte a skip is classified by. A
// quoted word and the two trailing forms are no longer a refusal.
func firstRefused(command string) int {
	core, _ := cutShellTail(strings.TrimRight(command, " "))
	words, bad := lexQuoted(core)
	if bad < 0 {
		_, bad = plainAssignments(words)
	}
	return bad
}
