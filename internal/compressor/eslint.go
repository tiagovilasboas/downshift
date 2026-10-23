// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// ESLint "stylish" formatter output (default):
//
//	/abs/path/to/file.ts
//	  12:5   error    Unexpected console statement  no-console
//	  18:1   warning  Missing return type            @typescript-eslint/explicit-...
//
//	✖ 3 problems (2 errors, 1 warning)
var (
	// A bare file path header line (no leading spaces, not a violation or summary).
	reEslintFile = regexp.MustCompile(`^(?:[./~]|[A-Za-z]:\\)\S*$`)
	// "  12:5  error  message  rule-name"
	reEslintViolation = regexp.MustCompile(`^\s+(\d+):(\d+)\s+(error|warning)\s+.+?\s+([@A-Za-z0-9/_-]+)\s*$`)
	// "✖ 3 problems (2 errors, 1 warning)"
	reEslintSummary = regexp.MustCompile(`(?m)^[✖✗x]?\s*(\d+)\s+problems?\s+\((\d+)\s+errors?,\s*(\d+)\s+warnings?\)`)
)

func isEslint(input []byte) bool {
	return reEslintSummary.Match(input)
}

// compressEslint groups ESLint stylish violations by file. It preserves the
// non-zero signal by reporting error/warning counts per file and the rules
// involved. Unrecognized transcripts are returned byte-for-byte.
func compressEslint(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00\x1b") {
		return input, false, "eslint_invalid_or_incomplete_input"
	}
	if !reEslintSummary.Match(input) {
		return input, false, "eslint_no_summary"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newViolationGroups()
	currentFile := ""
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " ")
		if trimmed == "" {
			currentFile = ""
			continue
		}
		if m := reEslintViolation.FindStringSubmatch(line); m != nil {
			if currentFile == "" {
				return input, false, "eslint_violation_without_file"
			}
			groups.add(currentFile, m[3] == "error", m[4])
			continue
		}
		if reEslintFile.MatchString(trimmed) {
			currentFile = trimmed
			groups.ensure(currentFile)
			continue
		}
		if reEslintSummary.MatchString(line) {
			currentFile = ""
			continue
		}
		// Any other line (stack trace, config error) means this is not a
		// clean stylish transcript; preserve everything.
		return input, false, "eslint_unrecognized_line"
	}

	if groups.fileCount() == 0 {
		return input, false, "eslint_no_violations_grouped"
	}

	out := groups.render("eslint")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed eslint (%d files)", groups.fileCount())
}

// violationGroups accumulates linter violations grouped by file in a stable,
// deterministic order. Shared by all linter compressors.
type violationGroups struct {
	order  []string
	byFile map[string]*fileViolations
}

type fileViolations struct {
	errors   int
	warnings int
	rules    []string
	seenRule map[string]bool
}

func newViolationGroups() *violationGroups {
	return &violationGroups{byFile: make(map[string]*fileViolations)}
}

func (g *violationGroups) ensure(file string) *fileViolations {
	fv, ok := g.byFile[file]
	if !ok {
		fv = &fileViolations{seenRule: make(map[string]bool)}
		g.byFile[file] = fv
		g.order = append(g.order, file)
	}
	return fv
}

func (g *violationGroups) add(file string, isError bool, rule string) {
	fv := g.ensure(file)
	if isError {
		fv.errors++
	} else {
		fv.warnings++
	}
	if rule != "" && !fv.seenRule[rule] {
		fv.seenRule[rule] = true
		fv.rules = append(fv.rules, rule)
	}
}

func (g *violationGroups) fileCount() int {
	n := 0
	for _, f := range g.order {
		fv := g.byFile[f]
		if fv.errors > 0 || fv.warnings > 0 {
			n++
		}
	}
	return n
}

// render produces one compact line per file:
//
//	file.ts: 2 errors, 1 warning (no-console, eqeqeq)
//
// followed by a total line. Files with no violations are skipped.
func (g *violationGroups) render(tool string) []byte {
	var buf strings.Builder
	totalErr, totalWarn := 0, 0
	for _, file := range g.order {
		fv := g.byFile[file]
		if fv.errors == 0 && fv.warnings == 0 {
			continue
		}
		totalErr += fv.errors
		totalWarn += fv.warnings
		buf.WriteString(file)
		buf.WriteString(": ")
		buf.WriteString(pluralCount(fv.errors, "error", "errors"))
		if fv.warnings > 0 {
			buf.WriteString(", ")
			buf.WriteString(pluralCount(fv.warnings, "warning", "warnings"))
		}
		if len(fv.rules) > 0 {
			rules := append([]string(nil), fv.rules...)
			sort.Strings(rules)
			buf.WriteString(" (")
			buf.WriteString(strings.Join(rules, ", "))
			buf.WriteString(")")
		}
		buf.WriteString("\n")
	}
	fmt.Fprintf(&buf, "%s: %d errors, %d warnings across %d files\n", tool, totalErr, totalWarn, g.fileCount())
	return []byte(buf.String())
}

func pluralCount(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
