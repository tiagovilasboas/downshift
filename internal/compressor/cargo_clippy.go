// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// cargo clippy output (rustc diagnostic format):
//
//	warning: unused variable: `x`
//	  --> src/main.rs:4:9
//	   |
//	4  |     let x = 5;
//	   |         ^ help: if this is intentional...
//	   |
//	   = note: `#[warn(unused_variables)]` on by default
//
//	error: this `if` has identical blocks
//	  --> src/lib.rs:10:5
//	...
//	warning: `mycrate` (bin "mycrate") generated 2 warnings
//
// The diagnostic header carries the level; the following "--> file:line:col"
// locates it. We group by that file and preserve the non-zero signal.
var (
	// "error: ..." / "warning: ..." / "error[E0308]: ..." header line.
	reClippyHeader = regexp.MustCompile(`^(error|warning)(?:\[[A-Za-z0-9]+\])?:\s+(.*)$`)
	// "  --> src/main.rs:4:9"
	reClippyLocation = regexp.MustCompile(`^\s*-->\s+(\S+?):(\d+):(\d+)\s*$`)
	// "warning: `crate` (bin "crate") generated 2 warnings" / "could not compile"
	reClippyGenerated = regexp.MustCompile(`(?m)^(?:error|warning):\s+.*\bgenerated\s+\d+\s+(?:warnings?|errors?)\b|^error: could not compile\b`)
)

func isCargoClippy(input []byte) bool {
	return reClippyGenerated.Match(input) && reClippyLocation.Match(input)
}

// compressCargoClippy groups clippy diagnostics by the file in their "-->"
// location line, preserving the error/warning split and the non-zero signal.
// The code frames and notes are dropped; unrecognized output passes through.
func compressCargoClippy(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00") {
		return input, false, "cargo_clippy_invalid_or_incomplete_input"
	}
	if !reClippyGenerated.Match(input) {
		return input, false, "cargo_clippy_no_summary"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newViolationGroups()
	pendingIsError := false
	havePending := false
	for _, line := range lines {
		if m := reClippyHeader.FindStringSubmatch(line); m != nil {
			// Skip the roll-up "generated N warnings" / aggregate lines.
			if reClippyGenerated.MatchString(line) {
				havePending = false
				continue
			}
			pendingIsError = m[1] == "error"
			havePending = true
			continue
		}
		if m := reClippyLocation.FindStringSubmatch(line); m != nil {
			if !havePending {
				// A location with no preceding header: treat as a secondary
				// span of the current diagnostic, not a new violation.
				continue
			}
			groups.add(m[1], pendingIsError, "")
			havePending = false
			continue
		}
		// Code frames, notes, help, blanks: ignored context.
	}

	if groups.fileCount() == 0 {
		return input, false, "cargo_clippy_no_violations_grouped"
	}

	out := groups.render("cargo clippy")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed cargo clippy (%d files)", groups.fileCount())
}
