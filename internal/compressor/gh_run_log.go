// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// gh run view --log compressor.
//
// The log output is extremely verbose (100KB-500KB typical, can reach several MB)
// because it includes every setup step, cache operations, environment variables,
// and verbose action output. An agent only needs:
//   - Which jobs ran and their status
//   - Errors and warnings (##[error], ##[warning])
//   - Test failures or build errors
//   - A summary of what happened
//
// Format: each line is tab-separated: JOB\tSTEP\tTIMESTAMP MESSAGE
// The timestamp is ISO8601 with a BOM marker on some lines.

var (
	// Matches the job/step/message format of gh run view --log
	reGhRunLogLine = regexp.MustCompile(`^([^\t]+)\t([^\t]+)\t(.*)$`)

	// GitHub Actions annotations that indicate problems
	reGhAnnotation = regexp.MustCompile(`##\[(error|warning|notice)\](.*)`)

	// Test failure patterns (Go, Jest, pytest, etc.)
	// Must be careful to not match "PASS" lines or test output that mentions "fail" as part of the test name
	reTestFailure = regexp.MustCompile(`(?m)^(?:FAIL\s|--- FAIL:|panic:|=== FAIL)`)

	// Build failure patterns - be specific to avoid false positives
	reBuildError = regexp.MustCompile(`(?i)^(?:error\[|error:|fatal:|cannot find module|undefined reference|compilation failed|build failed)`)

	// Skip these noisy setup steps
	ghRunLogSkipSteps = map[string]bool{
		"Set up job":                    true,
		"Post Run actions/checkout":     true,
		"Post Run actions/setup-go":     true,
		"Post Run actions/setup-node":   true,
		"Post Run actions/setup-python": true,
		"Post Run actions/cache":        true,
		"Complete job":                  true,
		"Run actions/checkout":          true,
	}

	// These step prefixes are setup noise
	ghRunLogSkipPrefixes = []string{
		"Post Run actions/",
		"Run actions/checkout",
		"Run actions/setup-",
		"Run actions/cache",
	}
)

// isGhRunLog detects `gh run view --log` output: multiple lines with
// JOB\tSTEP\tTIMESTAMP format.
func isGhRunLog(input []byte) bool {
	lines := strings.Split(string(input), "\n")
	if len(lines) < 10 {
		return false // Too short to be a CI log
	}

	matches := 0
	for _, line := range lines[:min(50, len(lines))] {
		if reGhRunLogLine.MatchString(line) {
			matches++
		}
	}
	// Need at least 5 matching lines in the first 50
	return matches >= 5
}

// compressGhRunLog extracts the essential information from verbose CI logs:
// - Job summary (which jobs ran)
// - All errors and warnings
// - Test/build failures
// - Drops setup noise (checkout, cache, env dumps)
func compressGhRunLog(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")

	type jobInfo struct {
		name      string
		steps     []string
		errors    []string
		warnings  []string
		important []string // test failures, build errors
		lineCount int
	}

	jobs := make(map[string]*jobInfo)
	var jobOrder []string

	for _, line := range lines {
		m := reGhRunLogLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		jobName := m[1]
		stepName := m[2]
		message := m[3]

		// Get or create job
		job, exists := jobs[jobName]
		if !exists {
			job = &jobInfo{name: jobName}
			jobs[jobName] = job
			jobOrder = append(jobOrder, jobName)
		}
		job.lineCount++

		// Skip noisy setup steps
		if ghRunLogSkipSteps[stepName] {
			continue
		}
		skip := false
		for _, prefix := range ghRunLogSkipPrefixes {
			if strings.HasPrefix(stepName, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		// Track unique steps
		if !containsString(job.steps, stepName) {
			job.steps = append(job.steps, stepName)
		}

		// Strip timestamp prefix (ISO8601 with optional BOM)
		msg := stripTimestamp(message)
		if msg == "" {
			continue
		}

		// Check for annotations (##[error], ##[warning])
		if am := reGhAnnotation.FindStringSubmatch(msg); am != nil {
			annotType := am[1]
			annotMsg := strings.TrimSpace(am[2])
			if annotMsg == "" {
				continue
			}
			switch annotType {
			case "error":
				if !containsString(job.errors, annotMsg) {
					job.errors = append(job.errors, annotMsg)
				}
			case "warning":
				// Skip common noise warnings
				if !isNoiseWarning(annotMsg) && !containsString(job.warnings, annotMsg) {
					job.warnings = append(job.warnings, annotMsg)
				}
			}
			continue
		}

		// Check for test/build failures in the message content
		if reTestFailure.MatchString(msg) || reBuildError.MatchString(msg) {
			// Truncate long error messages
			if len(msg) > 200 {
				msg = msg[:200] + "..."
			}
			if !containsString(job.important, msg) {
				job.important = append(job.important, msg)
			}
		}
	}

	if len(jobs) == 0 {
		return input, false, "gh_run_log_no_jobs_found"
	}

	// Build compact output
	var b strings.Builder
	fmt.Fprintf(&b, "=== CI Log Summary (%d jobs, %d lines compressed) ===\n\n", len(jobs), len(lines))

	totalErrors := 0
	totalWarnings := 0

	for _, jobName := range jobOrder {
		job := jobs[jobName]
		totalErrors += len(job.errors)
		totalWarnings += len(job.warnings)

		// Job header with step count
		status := "✓"
		if len(job.errors) > 0 || len(job.important) > 0 {
			status = "✗"
		}
		fmt.Fprintf(&b, "%s %s (%d steps, %d lines)\n", status, job.name, len(job.steps), job.lineCount)

		// List actual steps (not setup noise)
		if len(job.steps) > 0 && len(job.steps) <= 10 {
			for _, step := range job.steps {
				fmt.Fprintf(&b, "  - %s\n", step)
			}
		} else if len(job.steps) > 10 {
			for _, step := range job.steps[:5] {
				fmt.Fprintf(&b, "  - %s\n", step)
			}
			fmt.Fprintf(&b, "  ... and %d more steps\n", len(job.steps)-5)
		}

		// Errors (always show)
		for _, err := range job.errors {
			fmt.Fprintf(&b, "  ERROR: %s\n", err)
		}

		// Important messages (test failures, build errors)
		for _, msg := range job.important {
			fmt.Fprintf(&b, "  ISSUE: %s\n", msg)
		}

		// Warnings (limit to avoid noise)
		if len(job.warnings) > 0 {
			shown := job.warnings
			if len(shown) > 3 {
				shown = shown[:3]
			}
			for _, warn := range shown {
				fmt.Fprintf(&b, "  warn: %s\n", truncate(warn, 100))
			}
			if len(job.warnings) > 3 {
				fmt.Fprintf(&b, "  ... and %d more warnings\n", len(job.warnings)-3)
			}
		}

		b.WriteString("\n")
	}

	// Summary footer
	if totalErrors > 0 || totalWarnings > 0 {
		fmt.Fprintf(&b, "--- %d error(s), %d warning(s) ---\n", totalErrors, totalWarnings)
	} else {
		b.WriteString("--- No errors or warnings ---\n")
	}

	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}

	ratio := float64(len(out)) / float64(len(input)) * 100
	return []byte(out), true, fmt.Sprintf("compressed gh run log (%.1f%% of original, %d jobs, %d errors)", ratio, len(jobs), totalErrors)
}

// stripTimestamp removes the ISO8601 timestamp prefix from a log message.
// Format: 2026-10-24T12:54:19.3944192Z <message>
// Some lines have a BOM marker (U+FEFF) before the timestamp.
func stripTimestamp(s string) string {
	s = strings.TrimPrefix(s, "\ufeff") // Remove BOM
	s = strings.TrimSpace(s)

	// Match ISO8601 timestamp at start
	if len(s) > 28 && s[4] == '-' && s[7] == '-' && s[10] == 'T' {
		// Find the end of timestamp (after the Z)
		if idx := strings.Index(s, "Z "); idx > 0 && idx < 30 {
			return strings.TrimSpace(s[idx+2:])
		}
		if idx := strings.Index(s, "Z"); idx > 0 && idx < 30 && idx == len(s)-1 {
			return "" // Just a timestamp, no message
		}
	}
	return s
}

// isNoiseWarning returns true for common warnings that don't need agent attention.
func isNoiseWarning(msg string) bool {
	noise := []string{
		"ubuntu-latest label will migrate",
		"Node.js 16 actions are deprecated",
		"set-output command is deprecated",
		"save-state command is deprecated",
		"group command is deprecated",
	}
	for _, n := range noise {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
