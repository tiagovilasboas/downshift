// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"
)

// PHPStan and Psalm static analysis output compressor.
// Groups errors by file, similar to eslint/ruff pattern.
// RTK-style: "file.php: 3 errors (Rule1, Rule2)"

var (
	// PHPStan patterns (default table format)
	// ------ ----------------------------------------
	//  Line   path/to/File.php
	// ------ ----------------------------------------
	//   12    Error message here
	rePHPStanFile     = regexp.MustCompile(`(?m)^\s*Line\s+(.+\.php)\s*$`)
	rePHPStanError    = regexp.MustCompile(`(?m)^\s*(\d+)\s+(.+)$`)
	rePHPStanSummary  = regexp.MustCompile(`(?m)\[(?:OK|ERROR)\] Found (\d+) errors?`)
	rePHPStanNoErrors = regexp.MustCompile(`(?m)\[OK\] No errors`)

	// Psalm patterns
	// ERROR: TypeMismatch - path/to/File.php:12:5 - Message
	rePsalmError   = regexp.MustCompile(`^(ERROR|INFO|WARNING): (\w+) - (.+?):(\d+)(?::\d+)? - (.+)$`)
	rePsalmSummary = regexp.MustCompile(`(?m)^(\d+) errors? found`)
)

// isPHPStan detects PHPStan output.
func isPHPStan(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return rePHPStanSummary.Match(input) || rePHPStanNoErrors.Match(input) || bytes.Contains(input, []byte("PHPStan"))
}

// isPsalm detects Psalm output.
func isPsalm(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return rePsalmError.Match(input) || rePsalmSummary.Match(input) || bytes.Contains(input, []byte("Psalm"))
}

// compressPHPStan compacts PHPStan output to grouped errors.
func compressPHPStan(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "phpstan_too_large_or_invalid"
	}

	// Check for no errors case
	if rePHPStanNoErrors.Match(input) {
		return []byte("✓ phpstan: no errors\n"), true, "phpstan_clean"
	}

	lines := strings.Split(string(input), "\n")
	groups := newViolationGroups()
	var currentFile string
	var totalErrors int

	// Extract total from summary
	if m := rePHPStanSummary.FindSubmatch(input); m != nil {
		totalErrors = atoi(string(m[1]))
	}

	for _, line := range lines {
		// File header
		if m := rePHPStanFile.FindStringSubmatch(line); m != nil {
			currentFile = strings.TrimSpace(m[1])
			continue
		}

		// Error line
		if currentFile != "" {
			if m := rePHPStanError.FindStringSubmatch(line); m != nil {
				msg := strings.TrimSpace(m[2])
				// Extract rule-like identifier from message if present
				rule := extractPHPStanRule(msg)
				groups.add(currentFile, true, rule)
			}
		}
	}

	if groups.fileCount() == 0 {
		if totalErrors > 0 {
			// Couldn't parse details but have summary
			var out bytes.Buffer
			out.WriteString("✗ phpstan: ")
			out.WriteString(itoa(totalErrors))
			out.WriteString(" errors (details not parsed)\n")
			return out.Bytes(), true, "phpstan_summary_only"
		}
		return input, false, "phpstan_no_errors_found"
	}

	return groups.render("phpstan"), true, "phpstan_compacted"
}

// extractPHPStanRule tries to extract a rule-like identifier from PHPStan message
func extractPHPStanRule(msg string) string {
	// PHPStan messages often start with identifier-like text
	// e.g., "Parameter $x of method ... expects string, int given"
	// or "Call to undefined method ..."
	words := strings.Fields(msg)
	if len(words) >= 3 {
		// Use first few meaningful words as pseudo-rule
		key := strings.ToLower(words[0])
		if key == "parameter" || key == "call" || key == "method" || key == "property" || key == "class" || key == "variable" {
			return key + "_" + strings.ToLower(words[2])
		}
	}
	return "analysis"
}

// compressPsalm compacts Psalm output to grouped errors.
func compressPsalm(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "psalm_too_large_or_invalid"
	}

	lines := strings.Split(string(input), "\n")
	groups := newViolationGroups()
	var errorCount, warningCount int

	for _, line := range lines {
		if m := rePsalmError.FindStringSubmatch(line); m != nil {
			severity := strings.ToLower(m[1])
			rule := m[2]
			file := m[3]

			switch severity {
			case "error":
				groups.add(file, true, rule)
				errorCount++
			case "warning":
				groups.add(file, false, rule)
				warningCount++
			}
		}
	}

	if errorCount == 0 && warningCount == 0 {
		if bytes.Contains(input, []byte("No errors found!")) || bytes.Contains(input, []byte("0 errors")) {
			return []byte("✓ psalm: no errors\n"), true, "psalm_clean"
		}
		return input, false, "psalm_no_errors_found"
	}

	return groups.render("psalm"), true, "psalm_compacted"
}

// FormatPHPStan constant
const FormatPHPStan Format = "phpstan"

// FormatPsalm constant
const FormatPsalm Format = "psalm"
