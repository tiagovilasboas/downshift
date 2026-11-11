// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Pass lines whose duration is not the "... in <time>)" tail rePassTiming
// handles. Each regexp matches only the exact text its compressor writes, and
// capture group 1 is the part that is kept.
var (
	// rspec.go: "PASS rspec: N examples, N failures[, N pending] (1.23s)".
	rePassRspecTiming = lazyre.New(`^(PASS rspec: \d+ examples, \d+ failures(?:, \d+ pending)?) \(\d+(?:\.\d+)?s\)$`)
	// dotnet.go: "PASS dotnet test: N tests (1.2s)" and
	// "PASS dotnet test: N passed[, N skipped] / N total (1.2s)".
	rePassDotnetTiming = lazyre.New(`^(PASS dotnet test: (?:\d+ tests|\d+ passed(?:, \d+ skipped)? / \d+ total)) \(\d+(?:\.\d+)?s\)$`)
	// cypress.go: "...N skipped[; spec X]; 8 seconds)".
	rePassCypressTiming = lazyre.New(`^(Cypress: PASS \(\d+ tests: \d+ passing, \d+ pending, \d+ skipped(?:; spec [^;]+)?); \d+ (?:seconds?|minutes?(?:, \d+ seconds?)?)\)$`)
	// go_verbose.go: "Go test -v: PASS (N passed, 0 failed, N skipped) in 0.42s".
	rePassGoVerboseTiming = lazyre.New(`^(Go test -v: PASS \(\d+ passed, 0 failed, \d+ skipped\)) in \d+(?:\.\d+)?s$`)
)

// passTimingRule is one family: the regexp for its first line and the text
// that closes the kept part (Cypress keeps its closing parenthesis).
type passTimingRule struct {
	re     *lazyre.Regexp
	suffix string
}

var passTimingRules = []passTimingRule{
	{rePassRspecTiming, ""},
	{rePassDotnetTiming, ""},
	{rePassCypressTiming, ")"},
	{rePassGoVerboseTiming, ""},
}

// trimFamilyTiming drops the duration from the first line when it is exactly
// the pass line of the RSpec, dotnet test, Cypress or go test -v compressor.
// Every later line is untouched.
func trimFamilyTiming(text string) (string, bool) {
	first, rest, hasRest := strings.Cut(text, "\n")
	for _, r := range passTimingRules {
		if m := r.re.FindStringSubmatch(first); m != nil {
			out := m[1] + r.suffix
			if hasRest {
				out += "\n" + rest
			}
			return out, true
		}
	}
	return text, false
}
