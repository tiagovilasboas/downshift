// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// RegroupMatches rewrites search rows ("path:line:text") so each run of
// consecutive rows from one file prints the path once, as a header, and each
// row below it keeps everything after the path byte for byte, one space deep:
//
//	internal/core/route.go
//	 12:func Route(req Request) Decision {
//	 40:	return d
//
// The rewrite is lossless: the header joined to each row is the original
// line, in the original order. It reports false, and the caller keeps text,
// when any line is not a row (a `--` separator, a context row, a binary-file
// notice, a blank line), when a path begins with a space or a tab (it would
// read as a row), or when the rewrite is not at least a quarter shorter.
func RegroupMatches(text []byte) ([]byte, bool) {
	body := strings.TrimSuffix(string(text), "\n")
	if body == "" {
		return text, false
	}
	var b strings.Builder
	last := ""
	for _, line := range strings.Split(body, "\n") {
		m := reGrepMatch.FindStringSubmatch(line)
		if m == nil || strings.HasPrefix(m[1], " ") || strings.HasPrefix(m[1], "\t") {
			return text, false
		}
		if m[1] != last {
			b.WriteString(m[1] + "\n")
			last = m[1]
		}
		b.WriteString(" " + line[len(m[1])+1:] + "\n")
	}
	out := b.String()
	if len(out)*4 > len(text)*3 {
		return text, false
	}
	return []byte(out), true
}
