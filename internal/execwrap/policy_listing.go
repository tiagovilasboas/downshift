// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: Filesystem listing and search — ls, find, grep (read-only); file
// reads are in policy_read.go
// ════════════════════════════════════════════════════════════════════════════

// Listing and search families. A form is compacted only when its output is a
// plain list of names (ls, find) or of match rows (grep). Any other form is
// not a family here: it is refused and never runs through compact.
const (
	FamilyLs   Family = "ls"
	FamilyFind Family = "find"
	FamilyGrep Family = "grep"
)

// lsFlags are the ls options that keep the output a list of names: the long
// form (l), one name per line (1), a slash on directories (p), and which
// entries appear and in what order (a, A, r, t, S). Recursion (R), sizes (s,
// h), columns (C, x), colour and every long option are refused.
const lsFlags = "aAl1prtS"

// findRefused are the find primaries that write a file or run a program: the
// owner's list. -ls, -printf, -print0 and -X change the output from a list of
// paths, so they are refused too.
var findRefused = map[string]bool{
	"-exec": true, "-execdir": true, "-ok": true, "-okdir": true, "-delete": true,
	"-fprint": true, "-fprint0": true, "-fprintf": true, "-fls": true,
	"-ls": true, "-printf": true, "-print0": true, "-X": true,
}

// listingFamily names the ls, find, grep or file-read family of argv when its
// form is one the compactor reads, and FamilyNone otherwise.
func listingFamily(argv []string) Family {
	switch argv[0] {
	case "ls":
		if lsPlainForm(argv[1:]) {
			return FamilyLs
		}
	case "find":
		if findPlainForm(argv[1:]) {
			return FamilyFind
		}
	case "grep":
		if grepPlainForm(argv[1:]) {
			return FamilyGrep
		}
	case "cat", "head", "tail", "sed", "wc", "jq":
		return readFamily(argv)
	}
	return ripgrepFamily(argv)
}

// lsPlainForm accepts the flags in lsFlags and at most one operand. Two or more
// operands print a "name:" header before each directory, which is not a list.
func lsPlainForm(args []string) bool {
	operands := 0
	for _, a := range args {
		switch {
		case a == "-" || strings.HasPrefix(a, "--"):
			return false
		case strings.HasPrefix(a, "-"):
			if !onlyFlags(a[1:], lsFlags) {
				return false
			}
		default:
			operands++
		}
	}
	return operands <= 1
}

// findPlainForm accepts any find expression that carries none of the refused
// primaries. An unknown primary makes find fail, and a failed run stays raw.
func findPlainForm(args []string) bool {
	for _, a := range args {
		if findRefused[a] {
			return false
		}
	}
	return true
}
