// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// mypy default text output:
//
//	app/main.py:10: error: Incompatible return value type  [return-value]
//	app/main.py:22: error: Name "foo" is not defined  [name-defined]
//	app/util.py:3: note: ... (notes are context, not violations)
//	Found 2 errors in 1 file (checked 5 source files)
//
// Error codes appear in trailing brackets when `--show-error-codes` is on.
var (
	// "path/file.py:10: error: message  [code]"  (column optional: ":10:5:")
	reMypyViolation = regexp.MustCompile(`^(\S+?):(\d+):(?:(\d+):)?\s+(error|warning):\s+.*?(?:\[([a-z-]+)\])?\s*$`)
	// "path/file.py:10: note: ..." — context, ignored but allowed.
	reMypyNote = regexp.MustCompile(`^(\S+?):(\d+):(?:(\d+):)?\s+note:\s+.*$`)
	// "Found 2 errors in 1 file (checked 5 source files)"
	reMypySummary = regexp.MustCompile(`(?m)^Found\s+(\d+)\s+errors?\s+in\s+\d+\s+files?\b`)
)

func isMypy(input []byte) bool {
	return reMypySummary.Match(input) && reMypyViolation.Match(input)
}

// compressMypy groups mypy type errors by file, preserving error codes where
// present and the non-zero signal. Unrecognized transcripts pass through.
func compressMypy(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00") {
		return input, false, "mypy_invalid_or_incomplete_input"
	}
	if !reMypySummary.Match(input) {
		return input, false, "mypy_no_summary"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newViolationGroups()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := reMypyViolation.FindStringSubmatch(line); m != nil {
			rule := m[5] // may be empty when --show-error-codes is off
			groups.add(m[1], m[4] == "error", rule)
			continue
		}
		if reMypyNote.MatchString(line) {
			// Notes are supplementary context; ensure the file exists but do
			// not count them as violations.
			continue
		}
		if reMypySummary.MatchString(line) {
			continue
		}
		return input, false, "mypy_unrecognized_line"
	}

	if groups.fileCount() == 0 {
		return input, false, "mypy_no_violations_grouped"
	}

	out := groups.render("mypy")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed mypy (%d files)", groups.fileCount())
}
