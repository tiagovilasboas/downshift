// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: grep — match rows, file lists (-l) and counts (-c) (read-only)
// ════════════════════════════════════════════════════════════════════════════

// grepFlags are the grep options that keep one match per row and change no
// match text, plus -l (file names) and -c (counts per file). Context (A, B,
// C), files without a match (L), only-matching (o), quiet (q), inverted (v),
// byte offsets (b), tabs (T), NUL output (Z), -h (no file name) and every
// long option are refused.
const grepFlags = "rRnHiwxEFGlc"

// grepArgs is a parsed grep command line.
type grepArgs struct {
	flags    string   // every short flag letter, in order
	operands []string // paths, after the pattern when it is an operand
}

func (g grepArgs) has(letters string) bool { return strings.ContainsAny(g.flags, letters) }

// parseGrep splits args into flags and path operands, or reports false when a
// word is a long option, "-", or a flag outside grepFlags. With -e the pattern
// comes from the next word and every operand is a path; otherwise the first
// operand is the pattern.
func parseGrep(args []string) (grepArgs, bool) {
	var g grepArgs
	var operands []string
	explicit := false // -e gave the pattern, so no operand is one
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "-" || strings.HasPrefix(a, "--") {
			return g, false
		}
		if !strings.HasPrefix(a, "-") {
			operands = append(operands, a)
			continue
		}
		cluster := a[1:]
		if n := strings.IndexByte(cluster, 'e'); n >= 0 {
			// -e takes the pattern from the next argument and must end its cluster.
			if n != len(cluster)-1 || i+1 == len(args) {
				return g, false
			}
			explicit = true
			cluster = cluster[:n]
			i++
		}
		if !onlyFlags(cluster, grepFlags) {
			return g, false
		}
		g.flags += cluster
	}
	if !explicit {
		if len(operands) == 0 {
			return g, false
		}
		operands = operands[1:]
	}
	g.operands = operands
	return g, true
}

// grepPlainForm accepts a grep whose output is one of three plain shapes:
// file names (-l), `file:count` rows (-c) or one match per row, with a line
// number (-n) or without. Every row needs a file name: -H, -r, or two or more
// paths. A path is needed unless -r is set, since without one grep reads
// standard input. -l with -c is refused.
func grepPlainForm(args []string) bool {
	g, ok := parseGrep(args)
	if !ok || (g.has("l") && g.has("c")) {
		return false
	}
	recursive := g.has("rR")
	if !recursive && len(g.operands) == 0 {
		return false
	}
	if g.has("l") {
		return true
	}
	return g.has("H") || recursive || len(g.operands) > 1
}

// onlyFlags reports whether every letter of a flag cluster is in allowed.
func onlyFlags(cluster, allowed string) bool {
	for i := 0; i < len(cluster); i++ {
		if strings.IndexByte(allowed, cluster[i]) < 0 {
			return false
		}
	}
	return true
}
