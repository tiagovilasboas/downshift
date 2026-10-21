// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
)

// .NET test output patterns
var (
	// "Passed!  - Failed:     0, Passed:    42, Skipped:     0, Total:    42"
	reDotnetTestSummary = regexp.MustCompile(`(?m)^(Passed!|Failed!)\s+-\s+Failed:\s*(\d+),\s*Passed:\s*(\d+),\s*Skipped:\s*(\d+),\s*Total:\s*(\d+)`)
	// "Test run for /path/to/project.dll (.NETCoreApp,Version=v8.0)"
	reDotnetTestHeader = regexp.MustCompile(`(?m)^Test run for\s+`)
	// "Total tests: 42"
	reDotnetTestTotal = regexp.MustCompile(`(?m)^Total tests:\s*(\d+)`)
	// Duration: "Duration: 1.2345 s"
	reDotnetTestDuration = regexp.MustCompile(`(?m)Duration:\s*(\d+(?:\.\d+)?)\s*s`)
	// Detect failures
	reDotnetTestFailure = regexp.MustCompile(`(?m)(?:^Failed!|^\s*Failed\s+|^\s*X\s+|Error Message:|Stack Trace:)`)
)

// isDotnetTest returns true if input looks like dotnet test output.
func isDotnetTest(input []byte) bool {
	return reDotnetTestHeader.Match(input) || reDotnetTestSummary.Match(input)
}

// compressDotnetTest compresses successful dotnet test output to a summary.
func compressDotnetTest(input []byte) ([]byte, bool, string) {
	// Fail-open: if failures detected, preserve everything
	if reDotnetTestFailure.Match(input) {
		return input, false, "dotnet_test_failure_preserved"
	}

	// Try to extract summary
	summaryMatch := reDotnetTestSummary.FindSubmatch(input)
	if summaryMatch == nil {
		// Try simpler format
		totalMatch := reDotnetTestTotal.FindSubmatch(input)
		if totalMatch == nil {
			return input, false, "dotnet_test_no_summary"
		}
		// Simple format: just total
		total := string(totalMatch[1])
		duration := "?"
		if m := reDotnetTestDuration.FindSubmatch(input); m != nil {
			duration = string(m[1]) + "s"
		}
		return []byte(fmt.Sprintf("PASS dotnet test: %s tests (%s)\n", total, duration)), true, "dotnet_test_compacted"
	}

	// Full summary format
	status := string(summaryMatch[1])
	failed := string(summaryMatch[2])
	passed := string(summaryMatch[3])
	skipped := string(summaryMatch[4])
	total := string(summaryMatch[5])

	duration := "?"
	if m := reDotnetTestDuration.FindSubmatch(input); m != nil {
		duration = string(m[1]) + "s"
	}

	var buf bytes.Buffer
	if status == "Passed!" {
		buf.WriteString("PASS")
	} else {
		buf.WriteString("FAIL")
	}
	buf.WriteString(" dotnet test: ")
	buf.WriteString(passed)
	buf.WriteString(" passed")
	if skipped != "0" {
		buf.WriteString(", ")
		buf.WriteString(skipped)
		buf.WriteString(" skipped")
	}
	if failed != "0" {
		buf.WriteString(", ")
		buf.WriteString(failed)
		buf.WriteString(" failed")
	}
	buf.WriteString(fmt.Sprintf(" / %s total (%s)\n", total, duration))

	return buf.Bytes(), true, "dotnet_test_compacted"
}
