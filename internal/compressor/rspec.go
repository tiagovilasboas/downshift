// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
)

// RSpec output patterns
var (
	// "Finished in 0.12345 seconds (files took 1.23 seconds to load)"
	reRspecFinished = regexp.MustCompile(`(?m)^Finished in (\d+(?:\.\d+)?) seconds?`)
	// "3 examples, 0 failures"
	reRspecSummary = regexp.MustCompile(`(?m)^(\d+) examples?, (\d+) failures?(?:, (\d+) pending)?`)
	// Detect failures
	reRspecFailure = regexp.MustCompile(`(?m)(?:^\s*Failures?:|^\s*\d+\)\s+|FAILED|Error:|RuntimeError)`)
	// Progress dots: "..." or "..F.." — a line of only dots, F, *, P
	reRspecProgress = regexp.MustCompile(`(?m)^[.FP*]+$`)
	// File path lines in verbose mode: "./spec/foo_spec.rb"
	reRspecVerboseLine = regexp.MustCompile(`(?m)^  [a-zA-Z]`)
)

// isRspec returns true if input looks like RSpec output.
func isRspec(input []byte) bool {
	return reRspecFinished.Match(input) || reRspecSummary.Match(input)
}

// compressRspec compresses successful RSpec output to a summary.
// Failures are preserved in full.
func compressRspec(input []byte) ([]byte, bool, string) {
	// Fail-open: if failures detected, preserve everything
	if reRspecFailure.Match(input) {
		return input, false, "rspec_failure_preserved"
	}

	// Extract summary
	summaryMatch := reRspecSummary.FindSubmatch(input)
	if summaryMatch == nil {
		return input, false, "rspec_no_summary"
	}

	examples := string(summaryMatch[1])
	failures := string(summaryMatch[2])
	pending := ""
	if len(summaryMatch) > 3 && len(summaryMatch[3]) > 0 {
		pending = string(summaryMatch[3])
	}

	// Extract duration
	duration := "?"
	if m := reRspecFinished.FindSubmatch(input); m != nil {
		duration = string(m[1]) + "s"
	}

	// Build compact output
	var buf bytes.Buffer
	buf.WriteString("PASS rspec: ")
	buf.WriteString(examples)
	buf.WriteString(" examples, ")
	buf.WriteString(failures)
	buf.WriteString(" failures")
	if pending != "" {
		buf.WriteString(", ")
		buf.WriteString(pending)
		buf.WriteString(" pending")
	}
	buf.WriteString(fmt.Sprintf(" (%s)\n", duration))

	return buf.Bytes(), true, "rspec_compacted"
}
