// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"path/filepath"
	"strings"
)

// StripNoise makes a compacted result cheaper to read without removing a
// fact the agent can act on: an absolute path under cwd loses its cwd prefix
// (`/Users/x/repo/src/a.ts:3` becomes `src/a.ts:3`), because the agent runs
// in that directory and resolves the relative path to the same file.
//
// It never touches a result that was not compacted, so a raw answer (non-zero
// exit, danger word, unknown shape) keeps its bytes. Inside a compacted
// result a line with a danger word or a secret-looking word stays as printed.
// cwd is a parameter so a caller decides what "here" means; an empty or
// root-like cwd strips nothing. Durations are left alone: every duration the
// compressors keep sits in a summary line that is part of their contract.
func StripNoise(r Result, cwd string) Result {
	if !r.Applied || r.Format == FormatGitDiff || r.Format == FormatGitShow {
		return r
	}
	prefixes := cwdPrefixes(cwd)
	text := string(r.Output)
	if !containsAny(text, prefixes) {
		return r
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if len(line) > maxRegexLine || keepLineRaw(line) {
			continue
		}
		for _, p := range prefixes {
			line = trimPrefixAtBoundaries(line, p)
		}
		lines[i] = line
	}
	out := strings.Join(lines, "\n")
	if len(out) < len(text) {
		r.Output, r.ReducedBytes = []byte(out), len(out)
	}
	return r
}

// keepLineRaw reports whether a line must keep its bytes: a danger word or a
// secret-looking word makes the line evidence, not noise.
func keepLineRaw(line string) bool {
	return genericHasDanger(line) || reGitDiffSecret.MatchString(line)
}

// cwdPrefixes returns the spellings of "cwd plus a separator" that can open
// an absolute path. A Windows cwd (drive letter) also matches the
// forward-slash form tools such as node print. Only the string is read, so
// the rule works on any OS; a root or relative cwd yields no prefix.
func cwdPrefixes(cwd string) []string {
	clean := strings.TrimRight(cwd, `/\`)
	switch {
	case strings.HasPrefix(cwd, "/") && clean != "":
		return []string{clean + "/"}
	case len(clean) > 2 && clean[1] == ':' && isASCIILetter(clean[0]):
		slash := filepath.ToSlash(strings.ReplaceAll(clean, `\`, "/"))
		return []string{clean + `\`, slash + "/"}
	}
	return nil
}

func isASCIILetter(c byte) bool { return c|0x20 >= 'a' && c|0x20 <= 'z' }

func containsAny(text string, subs []string) bool {
	for _, s := range subs {
		if strings.Contains(text, s) {
			return true
		}
	}
	return false
}

// trimPrefixAtBoundaries removes prefix wherever it starts a path: not glued
// to a preceding path or word character (so /other/Users/x/repo/ stays), and
// followed by a path character (so a bare directory line never turns into an
// empty one).
func trimPrefixAtBoundaries(line, prefix string) string {
	var b strings.Builder
	from := 0
	for {
		i := strings.Index(line[from:], prefix)
		if i < 0 {
			break
		}
		i += from
		end := i + len(prefix)
		if pathStartsAt(line, i) && end < len(line) && !isSpace(line[end]) {
			b.WriteString(line[from:i])
		} else {
			b.WriteString(line[from:end])
		}
		from = end
	}
	b.WriteString(line[from:])
	return b.String()
}

// pathStartsAt reports whether index i of s begins a path token.
func pathStartsAt(s string, i int) bool {
	if i == 0 {
		return true
	}
	c := s[i-1]
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return false
	}
	return !strings.ContainsRune("_-./\\~", rune(c))
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\r' }
