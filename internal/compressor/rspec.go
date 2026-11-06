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
	reRspecFailure = regexp.MustCompile(`(?m)(?:^\s*Failures?:|^\s*\d+\)\s+|FAILED|Error:|RuntimeError|(?i:DEPRECATION))`)
)

// reRspecKnownLine matches the lines of a green run: the seed, the progress
// dots, the slowest-examples header, the timing and the example count. An
// indented line is known only inside the slowest-examples block.
var reRspecKnownLine = regexp.MustCompile(`^(Randomized with seed \d+|[.*]+|Top \d+ slowest example(?:s|(?: groups?)?) \(.*\):|Finished in .*|\d+ examples?, \d+ failures?(, \d+ pending)?)$`)

// reRspecSlowHeader opens the slowest-examples block; an indented line inside
// it (an example title or "    0.3 seconds ./spec/a_spec.rb:10") is known unless
// reRspecSlowNotice says it is a warning or deprecation.
var (
	reRspecSlowHeader = regexp.MustCompile(`^Top \d+ slowest example`)
	reRspecSlowNotice = regexp.MustCompile(`(?i)warn|deprecat`)
)

// rspecLinesRecognised reports whether every line is one of a green run's
// lines. Pending examples and every other unindented line are not summarised.
func rspecLinesRecognised(input []byte) bool {
	inSlowest := false
	for _, line := range bytes.Split(input, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			if !reRspecKnownLine.Match(line) {
				return false
			}
			inSlowest = reRspecSlowHeader.Match(line)
			continue
		}
		// An indented line outside the slowest block (a warning, a
		// deprecation) stays raw.
		if !inSlowest || reRspecSlowNotice.Match(line) {
			return false
		}
	}
	return true
}

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
	// Fail-open: only a transcript made of the lines of a green run is summarised.
	if !rspecLinesRecognised(input) {
		return input, false, "rspec_unrecognised_line_preserved"
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
