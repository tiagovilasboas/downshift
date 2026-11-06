// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"strings"
)

// Metachar classes: which shell metacharacter kept a command outside the
// safe-byte gate. The class names the first such byte; the log never holds the
// command text.
const (
	MetaSemicolon  = "semicolon"
	MetaAnd        = "and"
	MetaOr         = "or"
	MetaPipe       = "pipe"
	MetaBackground = "background"
	MetaSubst      = "subst"
	MetaBacktick   = "backtick"
	MetaRedirect   = "redirect"
	MetaQuote      = "quote"
	MetaNewline    = "newline"
	MetaGlob       = "glob"
	MetaBackslash  = "backslash"
	MetaParen      = "paren"
	MetaOther      = "other"
)

// MetacharClasses lists every class, for tests and the docs.
var MetacharClasses = []string{
	MetaSemicolon, MetaAnd, MetaOr, MetaPipe, MetaBackground, MetaSubst,
	MetaBacktick, MetaRedirect, MetaQuote, MetaNewline, MetaGlob,
	MetaBackslash, MetaParen, MetaOther,
}

// MetacharClass returns the class of the first byte of command the policy
// still refuses, or "" when there is none.
func MetacharClass(command string) string {
	i := firstUnsafe(command)
	if i < 0 {
		return ""
	}
	next := byte(0)
	if i+1 < len(command) {
		next = command[i+1]
	}
	switch c := command[i]; c {
	case ';':
		return MetaSemicolon
	case '&':
		switch next {
		case '&':
			return MetaAnd
		case '>':
			return MetaRedirect
		}
		return MetaBackground
	case '|':
		if next == '|' {
			return MetaOr
		}
		return MetaPipe
	case '$':
		return MetaSubst
	case '`':
		return MetaBacktick
	case '<', '>':
		return MetaRedirect
	case '\'', '"':
		return MetaQuote
	case '\n', '\r':
		return MetaNewline
	case '*', '?', '[', ']', '{', '}', '~':
		return MetaGlob
	case '\\':
		return MetaBackslash
	case '(', ')':
		return MetaParen
	}
	return MetaOther
}

// firstUnsafe is the index of the first byte the policy still refuses, or -1.
// A closed quoted word and an exact trailing 2>/dev/null or || true are
// accepted (policy_quoted.go), so they are not what a skip is classified by.
func firstUnsafe(command string) int { return firstRefused(command) }

// SkipDetail returns the metacharacter class and the command family of a
// metachar skip, and "", "" for any other skip. The family is the one named by
// the words before the first metacharacter, "" when none is allowlisted. Only
// fixed codes are returned, never command text.
func SkipDetail(reason, command string) (class, family string) {
	if reason != SkipMetachar {
		return "", ""
	}
	i := firstUnsafe(command)
	if i < 0 {
		return "", ""
	}
	words := strings.Fields(command[:i])
	if lexed, bad := lexQuoted(command[:i]); bad < 0 {
		words = words[:0]
		for _, w := range lexed {
			words = append(words, w.text)
		}
	}
	_, argv := splitEnvAssignments(words)
	return MetacharClass(command), string(FamilyOf(argv))
}

// SummarizeSkipClasses counts the metachar skips of the log by class. Rows
// written before the class existed count under no class. A missing file is
// empty.
func SummarizeSkipClasses(p string) (map[string]int64, error) {
	var out map[string]int64
	err := scanLog(p, func(line []byte) {
		var ev SkipEvent
		if json.Unmarshal(line, &ev) != nil || ev.Metachar == "" {
			return
		}
		if out == nil {
			out = map[string]int64{}
		}
		out[ev.Metachar]++
	})
	return out, err
}

// SummarizeSkipKinds counts the metachar skips of the log by MetacharKind.
// Rows written before the kind existed count under none. A missing file is
// empty.
func SummarizeSkipKinds(p string) (map[string]int64, error) {
	var out map[string]int64
	err := scanLog(p, func(line []byte) {
		var ev SkipEvent
		if json.Unmarshal(line, &ev) != nil || ev.Kind == "" {
			return
		}
		if out == nil {
			out = map[string]int64{}
		}
		out[ev.Kind]++
	})
	return out, err
}
