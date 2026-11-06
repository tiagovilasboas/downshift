// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"regexp"
	"strconv"
	"strings"
)

// ════════════════════════════════════════════════════════════════════════════
// DOMAIN: reading one file — cat, head, tail, sed -n, wc -l (read-only)
// ════════════════════════════════════════════════════════════════════════════

// File-read families. Only a plain read of named files is a family: no
// option that follows, numbers lines, joins streams or writes, and no
// standard input. Whether an operand is a regular file is checked when the
// command runs (policy_read_files.go), since the argv alone cannot say.
const (
	FamilyFileRead Family = "file-read"
	FamilyWc       Family = "wc"
)

// reSedRange is the one sed script accepted: print lines A to B (or line A).
var reSedRange = regexp.MustCompile(`^([1-9][0-9]{0,8})(?:,([1-9][0-9]{0,8}))?p$`)

// maxLineCount bounds head and tail -n so a count cannot overflow.
const maxLineCount = 1_000_000

// readFamily names the file-read family of argv, or FamilyNone.
//
//	cat FILE
//	head|tail [-n N | -nN | -N] FILE        (no -f, -F, -c, -q, +N or negative N)
//	sed -n 'A,Bp' FILE                      (no -i, -e, -f, -s, no other script)
//	wc -l FILE...
//	jq . FILE                               (the identity filter only, see policy_json.go)
func readFamily(argv []string) Family {
	args := argv[1:]
	switch argv[0] {
	case "cat":
		if len(args) == 1 && plainFileOperand(args[0]) {
			return FamilyFileRead
		}
	case "head", "tail":
		if _, file, ok := headTailArgs(args); ok && plainFileOperand(file) {
			return FamilyFileRead
		}
	case "sed":
		if _, _, file, ok := sedRangeArgs(args); ok && plainFileOperand(file) {
			return FamilyFileRead
		}
	case "wc":
		if wcListArgs(args) {
			return FamilyWc
		}
	case "jq":
		if file, ok := jqIdentityArgs(args); ok && plainFileOperand(file) {
			return FamilyFileRead
		}
	}
	return FamilyNone
}

// plainFileOperand rejects what is not the name of a file on disk: an empty
// word, an option or "+N" (a flag spelling), "-" (standard input) and the
// device, process and kernel trees, whose "files" never end or change.
func plainFileOperand(s string) bool {
	if s == "" || s[0] == '-' || s[0] == '+' || strings.ContainsRune(s, 0) {
		return false
	}
	for _, tree := range []string{"/dev", "/proc", "/sys"} {
		if s == tree || strings.HasPrefix(s, tree+"/") {
			return false
		}
	}
	return true
}

// headTailArgs reads `[-n N | -nN | -N] FILE` and returns the line count (0
// when none was given) and the file. Anything else is not ok.
func headTailArgs(args []string) (n int, file string, ok bool) {
	switch len(args) {
	case 1:
		return 0, args[0], true
	case 2:
		count := strings.TrimPrefix(args[0], "-")
		if strings.HasPrefix(args[0], "-n") {
			count = args[0][2:]
		}
		n, ok = lineCount(count)
		return n, args[1], ok && strings.HasPrefix(args[0], "-")
	case 3:
		n, ok = lineCount(args[1])
		return n, args[2], ok && args[0] == "-n"
	}
	return 0, "", false
}

// lineCount parses a positive decimal count of at most maxLineCount.
func lineCount(s string) (int, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > maxLineCount || strings.TrimLeft(s, "0123456789") != "" {
		return 0, false
	}
	return n, true
}

// sedRangeArgs reads `-n 'A,Bp' FILE` (or 'Ap') and returns A, B and the file.
func sedRangeArgs(args []string) (from, to int, file string, ok bool) {
	if len(args) != 3 || args[0] != "-n" {
		return 0, 0, "", false
	}
	m := reSedRange.FindStringSubmatch(args[1])
	if m == nil {
		return 0, 0, "", false
	}
	from, _ = strconv.Atoi(m[1])
	to = from
	if m[2] != "" {
		to, _ = strconv.Atoi(m[2])
	}
	return from, to, args[2], to >= from
}

// wcListArgs reads `-l FILE...`: the line count only, of one or more files.
func wcListArgs(args []string) bool {
	if len(args) < 2 || args[0] != "-l" {
		return false
	}
	for _, f := range args[1:] {
		if !plainFileOperand(f) {
			return false
		}
	}
	return true
}
