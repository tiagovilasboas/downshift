// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// cargo build output (rustc diagnostic format):
//
//	   Compiling myapp v0.1.0 (/path/to/myapp)
//	warning: unused variable: `x`
//	  --> src/main.rs:4:9
//	   |
//	4  |     let x = 5;
//	   |         ^ help: if this is intentional, prefix it with an underscore
//	   |
//	   = note: `#[warn(unused_variables)]` on by default
//
//	error[E0308]: mismatched types
//	  --> src/lib.rs:10:5
//	...
//	error: aborting due to 1 previous error; 1 warning emitted
//
// The compressor keeps only the diagnostic header and its "--> file:line:col"
// location, grouped by file, and drops the progress noise ("Compiling",
// "Downloading"), the code frames, the "|" rails and the "= note/help" context.
// Any line outside this known grammar makes the whole transcript pass through
// unchanged, so an unexpected shape never loses a fact.
var (
	// "error: ...", "error[E0308]: ...", "warning: ..." header line.
	reCargoBuildHeader = regexp.MustCompile(`^(error|warning)(?:\[[A-Za-z0-9]+\])?:\s+(.*)$`)
	// Multiline guard: at least one diagnostic header anywhere in the input.
	reCargoBuildHeaderM = regexp.MustCompile(`(?m)^(?:error|warning)(?:\[[A-Za-z0-9]+\])?:\s+`)
	// "  --> src/main.rs:4:9" location line.
	reCargoBuildLocation = regexp.MustCompile(`^\s*-->\s+(\S+?):(\d+):(\d+)\s*$`)
	// Multiline guard for the location line.
	reCargoBuildLocationM = regexp.MustCompile(`(?m)^\s*-->\s+\S+?:\d+:\d+\s*$`)
	// Progress / status lines cargo prints to stderr during a build.
	reCargoBuildProgress = regexp.MustCompile(`^\s*(?:Compiling|Checking|Downloading|Downloaded|Updating|Fresh|Building|Installing|Compiling|Blocking|Adding|Removing|Finished)\b`)
	// Aggregate roll-up lines: "error: aborting due to ...", "warning: `crate`
	// (bin "crate") generated 2 warnings", "error: could not compile ...".
	reCargoBuildAggregate = regexp.MustCompile(`^(?:error:\s+aborting due to\b|error:\s+could not compile\b|(?:error|warning):\s+.*\bgenerated\s+\d+\s+(?:warnings?|errors?)\b)`)
	// Code-frame context lines of a located diagnostic: the "N | source",
	// "  | ^^^ help", "  = note/help", and bare "  |" rails rustc prints.
	// Recognizing them explicitly means an arbitrary line inside a diagnostic
	// still breaks the grammar and preserves, rather than being swallowed.
	reCargoBuildContext = regexp.MustCompile(`^\s*(?:\d+\s*)?[|=]`)
)

// isCargoBuild returns true when the output carries at least one rustc
// diagnostic header and its location line: the fingerprint of cargo build.
func isCargoBuild(input []byte) bool {
	return reCargoBuildHeaderM.Match(input) && reCargoBuildLocationM.Match(input)
}

// compressCargoBuild groups cargo build errors and warnings by the file in
// their "-->" location line, preserving each diagnostic message so no fact is
// lost. Progress, code frames and notes are dropped. An input with any
// unrecognized line, or no diagnostics to group, is returned byte-for-byte.
func compressCargoBuild(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00") {
		return input, false, "cargo_build_invalid_or_incomplete_input"
	}
	if !reCargoBuildHeaderM.Match(input) {
		return input, false, "cargo_build_no_diagnostics"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newBuildDiagnostics()
	var pendingIsError bool
	var pendingMsg string
	havePending := false
	inContext := false // true while inside a diagnostic's code frame / notes

	for _, line := range lines {
		// Aggregate roll-up lines carry no new location; skip them.
		if reCargoBuildAggregate.MatchString(line) {
			havePending = false
			inContext = false
			continue
		}
		if m := reCargoBuildHeader.FindStringSubmatch(line); m != nil {
			pendingIsError = m[1] == "error"
			pendingMsg = line
			havePending = true
			inContext = false
			continue
		}
		if m := reCargoBuildLocation.FindStringSubmatch(line); m != nil {
			if !havePending {
				// A secondary span of the current diagnostic, not a new one.
				continue
			}
			groups.add(m[1], pendingIsError, pendingMsg)
			havePending = false
			inContext = true
			continue
		}
		if line == "" || reCargoBuildProgress.MatchString(line) {
			continue
		}
		// Only recognized rustc code-frame / note lines belong to a located
		// diagnostic. An arbitrary line, even inside a diagnostic, is outside
		// the known grammar and preserves the whole transcript.
		if (inContext || havePending) && reCargoBuildContext.MatchString(line) {
			continue
		}
		// Anything else is outside the known grammar: preserve everything.
		return input, false, "cargo_build_unrecognized_line"
	}

	if groups.fileCount() == 0 {
		return input, false, "cargo_build_no_diagnostics_grouped"
	}

	out := groups.render("cargo build")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed cargo build (%d files)", groups.fileCount())
}
