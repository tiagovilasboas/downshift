// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// Metachar kinds: a coarse, fixed-precedence split of the one undivided
// "metachar" skip bucket, so the shapes worth supporting can be counted before
// the grammar is widened. A kind is a code, never command text. It differs
// from the Meta* classes (skip_class.go), which name the first refused byte.
const (
	KindHeredoc    = "heredoc"
	KindSubst      = "subst"
	KindRedirect   = "redirect"
	KindOrList     = "or_list"
	KindBackground = "background"
	KindPipeGrep   = "pipe_grep"
	KindPipeOther  = "pipe_other"
	KindQuote      = "quote"
	KindGlob       = "glob"
	KindOther      = "other"
)

// MetacharKinds lists every kind in precedence order, for tests and the docs.
var MetacharKinds = []string{
	KindHeredoc, KindSubst, KindRedirect, KindOrList, KindBackground,
	KindPipeGrep, KindPipeOther, KindQuote, KindGlob, KindOther,
}

// kindScan is what one pass over the command found outside quotes (a
// substitution is also found inside double quotes, where the shell expands it).
type kindScan struct {
	heredoc, subst, redirect, orList, background bool
	pipe, pipeGrep                               bool
}

// MetacharKind returns the one kind of command, chosen by the fixed precedence
// heredoc, subst, redirect, or_list, background, pipe_grep, pipe_other, quote,
// glob, other. The text is read in memory only; only the code is returned.
func MetacharKind(command string) string {
	s := scanKinds(strings.TrimSpace(command))
	switch {
	case s.heredoc:
		return KindHeredoc
	case s.subst:
		return KindSubst
	case s.redirect:
		return KindRedirect
	case s.orList:
		return KindOrList
	case s.background:
		return KindBackground
	case s.pipeGrep:
		return KindPipeGrep
	case s.pipe:
		return KindPipeOther
	case strings.ContainsAny(command, `'"`):
		return KindQuote
	case strings.ContainsAny(command, "*?[~"):
		return KindGlob
	}
	return KindOther
}

// scanKinds walks command once, tracking quotes so a pipe, redirection or
// ampersand inside a quoted word does not count. A trailing exact 2>&1 is the
// one redirection that is not a kind.
func scanKinds(c string) kindScan {
	c = strings.TrimSuffix(c, "2>&1")
	var s kindScan
	var quote byte
	for i := 0; i < len(c); i++ {
		b := c[i]
		next, prev := byteAt(c, i+1), byteAt(c, i-1)
		switch {
		case b == '\\' && quote != '\'':
			i++
		case quote != 0:
			if b == quote {
				quote = 0
			} else if quote == '"' && (b == '`' || isSubstStart(b, next)) {
				s.subst = true
			}
		case b == '\'' || b == '"':
			quote = b
		case b == '`' || isSubstStart(b, next):
			s.subst = true
		case b == '<' && next == '<':
			s.heredoc = true
			i++
		case b == '<' || b == '>':
			s.redirect = true
		case b == '|' && next == '|':
			s.orList = true
			i++
		case b == '|':
			s.pipe = true
			s.pipeGrep = s.pipeGrep || nextStageIsGrep(c[i+1:])
		case b == '&' && next == '&':
			i++
		case b == '&' && prev != '>' && prev != '<':
			s.background = true
		}
	}
	return s
}

// isSubstStart reports a dollar sign that opens an expansion: a command or
// arithmetic substitution, a braced name or a plain variable name.
func isSubstStart(b, next byte) bool {
	if b != '$' {
		return false
	}
	return next == '(' || next == '{' || next == '_' ||
		next >= 'a' && next <= 'z' || next >= 'A' && next <= 'Z'
}

// nextStageIsGrep reports whether the pipe stage in rest starts with a
// grep-family word.
func nextStageIsGrep(rest string) bool {
	w := strings.Fields(rest)
	if len(w) == 0 {
		return false
	}
	switch w[0] {
	case "grep", "rg", "egrep", "fgrep":
		return true
	}
	return false
}

func byteAt(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}
