// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// golangci-lint default (line) output:
//
//	internal/foo/bar.go:12:6: `x` is unused (unused)
//	internal/foo/bar.go:30:2: ineffectual assignment to err (ineffassign)
//	cmd/main.go:8:1: File is not `gofmt`-ed (gofmt)
//
// Each line ends with the linter name in parentheses. golangci-lint does not
// print a trailing summary count by default, so the format fingerprint is the
// "(linter)" suffix on file:line:col rows.
var (
	// "path/file.go:12:6: message (linter)" — col may be absent.
	reGolangciViolation = regexp.MustCompile(`^(\S+\.go):(\d+):(?:(\d+):)?\s+.*\(([a-z0-9_-]+)\)\s*$`)
)

func isGolangciLint(input []byte) bool {
	return reGolangciViolation.Match(input)
}

// compressGolangciLint groups golangci-lint violations by file, preserving the
// linter names and the non-zero signal. Unrecognized transcripts pass through.
func compressGolangciLint(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00") {
		return input, false, "golangci_lint_invalid_or_incomplete_input"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newViolationGroups()
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := reGolangciViolation.FindStringSubmatch(line); m != nil {
			groups.add(m[1], true, m[4])
			continue
		}
		// Any non-violation line (level=warning banners, config errors) means
		// the transcript is not purely the line formatter; preserve it all.
		return input, false, "golangci_lint_unrecognized_line"
	}

	if groups.fileCount() == 0 {
		return input, false, "golangci_lint_no_violations_grouped"
	}

	out := groups.render("golangci-lint")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed golangci-lint (%d files)", groups.fileCount())
}
