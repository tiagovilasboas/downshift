// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Ruff "full"/default text output:
//
//	app/main.py:10:1: F401 [*] `os` imported but unused
//	app/main.py:22:5: E722 Do not use bare `except`
//	Found 2 errors.
//
// Each violation line carries file:line:col, a rule code, and a message.
var (
	// "path/file.py:10:1: F401 message..." — rule code is a letter run + digits.
	reRuffViolation = regexp.MustCompile(`^(\S+?):(\d+):(\d+):\s+([A-Z]+\d+)\s+.*$`)
	// Multiline detector for the whole-input scan (the per-line regex above is
	// anchored for the compressor loop).
	reRuffViolationDetect = regexp.MustCompile(`(?m)^(\S+?):(\d+):(\d+):\s+([A-Z]+\d+)\s+.*$`)
	// "Found 3 errors." / "Found 1 error." / "Found 3 errors (2 fixable...)."
	reRuffSummary = regexp.MustCompile(`(?m)^Found\s+(\d+)\s+errors?\b`)
)

func isRuff(input []byte) bool {
	return reRuffSummary.Match(input) && reRuffViolationDetect.Match(input)
}

// compressRuff groups ruff violations by file, preserving rule codes and the
// non-zero signal. Unrecognized transcripts are returned byte-for-byte.
func compressRuff(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00\x1b") {
		return input, false, "ruff_invalid_or_incomplete_input"
	}
	if !reRuffSummary.Match(input) {
		return input, false, "ruff_no_summary"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newViolationGroups()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := reRuffViolation.FindStringSubmatch(line); m != nil {
			// ruff reports all diagnostics as errors at the CLI level.
			groups.add(m[1], true, m[4])
			continue
		}
		if reRuffSummary.MatchString(line) {
			continue
		}
		// Diff/fix hints ("|", "^", "= help:") and anything else mean this is
		// not a clean compact transcript; preserve everything.
		return input, false, "ruff_unrecognized_line"
	}

	if groups.fileCount() == 0 {
		return input, false, "ruff_no_violations_grouped"
	}

	out := groups.render("ruff")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed ruff (%d files)", groups.fileCount())
}
