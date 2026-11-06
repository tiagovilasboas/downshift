// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "regexp"

// A package script (`npm test`, `pnpm test`) runs vitest or jest, and the
// command line does not say which. Each runner prints a summary line the other
// never does: vitest its "RUN v1.2.3" header, its "Test Files" count and its
// "⎯⎯⎯ Failed Tests" heading, jest its "Test Suites:" count. The FAIL and Tests lines are shared, so they decide
// nothing.
var (
	reVitestRunMark = regexp.MustCompile(`(?m)^\s*(?:RUN\s+v\d+\.\d+|Test Files\s+\d|⎯{3,} Failed Tests)`)
	reJestRunMark   = regexp.MustCompile(`(?m)^Test Suites:\s`)
)

// JSTestRunner reports which JavaScript test runner wrote a transcript:
// FormatVitest or FormatJest. It returns FormatUnknown when the text carries
// the marks of both runners, of neither, or cannot be read as plain text, so a
// caller that cannot tell keeps the output raw.
func JSTestRunner(input []byte) Format {
	text, reason := screenText(input)
	if reason != "" {
		return FormatUnknown
	}
	vitest, jest := reVitestRunMark.Match(text), reJestRunMark.Match(text)
	switch {
	case vitest && !jest:
		return FormatVitest
	case jest && !vitest:
		return FormatJest
	}
	return FormatUnknown
}
