// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Composer (PHP) install/update output compressor.
// Strips verbose download progress, keeps only the summary.
// RTK-style: "✓ composer: installed 45 packages in 12.3s"

var (
	// Composer summary patterns
	reComposerPackageOps = regexp.MustCompile(`Package operations: (\d+) installs?, (\d+) updates?, (\d+) removals?`)
	reComposerNothingTo  = regexp.MustCompile(`(?m)^Nothing to install,? (?:update|modify)`)
	reComposerLockFresh  = regexp.MustCompile(`(?m)^Installing dependencies from lock file`)
	reComposerGenerating = regexp.MustCompile(`(?m)^Generating (?:optimized )?autoload files`)
	reComposerRunScript  = regexp.MustCompile(`(?m)^> [\w@:/-]+`)
	reComposerWarning    = regexp.MustCompile(`(?i)warning|deprecated`)
	reComposerError      = regexp.MustCompile(`(?i)^  \[[\w\\]+Exception\]|fatal error|composer install failed`)
)

// isComposer detects composer install/update output.
func isComposer(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	// Look for characteristic composer patterns
	return reComposerPackageOps.Match(input) ||
		reComposerLockFresh.Match(input) ||
		reComposerNothingTo.Match(input) ||
		reComposerGenerating.Match(input)
}

// compressComposer compacts composer output to a summary line.
func compressComposer(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "composer_too_large_or_invalid"
	}

	// Fail open on errors
	if reComposerError.Match(input) {
		return input, false, "composer_has_error"
	}

	lines := strings.Split(string(input), "\n")

	var installs, updates, removals int
	var warnings []string
	var scripts []string
	var hasAutoload bool
	var nothingToDo bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Count operations from summary line
		if m := reComposerPackageOps.FindStringSubmatch(line); m != nil {
			installs = atoi(m[1])
			updates = atoi(m[2])
			removals = atoi(m[3])
			continue
		}

		// Nothing to do
		if reComposerNothingTo.MatchString(line) {
			nothingToDo = true
			continue
		}

		// Autoload generated
		if reComposerGenerating.MatchString(line) {
			hasAutoload = true
			continue
		}

		// Scripts run
		if reComposerRunScript.MatchString(line) {
			scripts = append(scripts, line)
			continue
		}

		// Warnings (keep but summarize)
		if reComposerWarning.MatchString(line) && len(line) > 5 {
			warnings = append(warnings, line)
		}
	}

	var out bytes.Buffer

	// Build summary
	if nothingToDo {
		out.WriteString("✓ composer: nothing to install/update")
	} else {
		total := installs + updates + removals
		if total == 0 && !hasAutoload {
			return input, false, "composer_no_ops_detected"
		}

		out.WriteString("✓ composer: ")
		var parts []string
		if installs > 0 {
			parts = append(parts, pluralCount(installs, "install", "installs"))
		}
		if updates > 0 {
			parts = append(parts, pluralCount(updates, "update", "updates"))
		}
		if removals > 0 {
			parts = append(parts, pluralCount(removals, "removal", "removals"))
		}
		if len(parts) > 0 {
			out.WriteString(strings.Join(parts, ", "))
		} else {
			out.WriteString("autoload regenerated")
		}
	}

	// Add autoload note if present
	if hasAutoload && !nothingToDo {
		out.WriteString(" + autoload")
	}
	out.WriteString("\n")

	// Show warnings summary
	if len(warnings) > 0 {
		if len(warnings) <= 3 {
			for _, w := range warnings {
				out.WriteString("⚠ ")
				out.WriteString(truncateLine(w, 100))
				out.WriteString("\n")
			}
		} else {
			out.WriteString("⚠ ")
			out.WriteString(itoa(len(warnings)))
			out.WriteString(" warnings (use -v for details)\n")
		}
	}

	// Show scripts run
	if len(scripts) > 0 {
		out.WriteString("scripts: ")
		out.WriteString(itoa(len(scripts)))
		out.WriteString(" executed\n")
	}

	return out.Bytes(), true, "composer_compacted"
}

// truncateLine truncates a line to maxLen, adding "..." if truncated
func truncateLine(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// atoi converts string to int, returns 0 on error
func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

// FormatComposer constant
const FormatComposer Format = "composer"
