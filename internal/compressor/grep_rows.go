// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"path"
	"strings"
)

// compressGrepRows groups `path:text` rows (grep -r or rg without -n, so no
// line numbers) under their file, with the caps of grep_caps.go. Unlike a
// numbered row, the text after the path may itself hold colons, and one file
// operand prints rows with no path at all, so a row counts only when the text
// before its first colon is one of the command's path operands or lies below
// one (CommandHint.Roots). Any other row, and an empty Roots, keep the output
// raw.
func compressGrepRows(input []byte, h CommandHint) ([]byte, bool, string) {
	if len(h.Roots) == 0 {
		return input, false, "grep_rows_no_roots_preserved"
	}
	g := newMatchGroups()
	for _, line := range commandLines(input) {
		file, text, ok := strings.Cut(line, ":")
		if !ok || !underRoot(file, h.Roots) {
			return input, false, "grep_unrecognized_structure_preserved"
		}
		g.add(file, strings.TrimLeft(text, " \t"))
	}
	out := []byte(g.render())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "grep rows grouped by file"
}

// underRoot reports whether p is root or a path below root. "." is every
// relative path the search printed with a "./" prefix.
func underRoot(p string, roots []string) bool {
	for _, r := range roots {
		r = path.Clean(r)
		switch {
		case p == r:
			return true
		case r == ".":
			if strings.HasPrefix(p, "./") && len(p) > 2 {
				return true
			}
		case strings.HasPrefix(p, strings.TrimSuffix(r, "/")+"/"):
			return true
		}
	}
	return false
}
