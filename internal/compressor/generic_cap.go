// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// The cap keeps the start of a long output (the command and its first
// results) and a longer tail (where the verdict usually is).
const (
	capHead = 50
	capTail = 200
	// capKeepMax bounds how many flagged lines survive from the cut region, so
	// a noisy log cannot defeat the cap.
	capKeepMax = 100
)

// reCapKeep matches lines in the cut region that must survive the cut: the
// words an agent acts on that the generic danger gate does not stop on
// (skipped tests, missing files, vulnerabilities, outdated or pending items),
// plus the gate's own words as a second line of defence.
var reCapKeep = lazyre.New(
	`(?i)skip|missing|vulnerab|outdated|pending|deprecat|warn|error|fail`)

// capLines keeps the first capHead and the last capTail lines of an output
// longer than both plus a marker line. The middle is replaced by
// "[N lines elided]", except that flagged lines (reCapKeep) of the middle stay
// in order, each gap between them carrying its own marker, so the markers sum
// to the number of lines cut. It reports whether it cut anything.
func capLines(lines []string) ([]string, bool) {
	if len(lines) <= capHead+capTail+2 {
		return lines, false
	}
	tailStart := len(lines) - capTail
	out := make([]string, 0, capHead+capTail+1)
	out = append(out, lines[:capHead]...)
	gap, kept := 0, 0
	for _, l := range lines[capHead:tailStart] {
		if kept < capKeepMax && reCapKeep.MatchString(l) {
			if gap > 0 {
				out = append(out, fmt.Sprintf("[%d lines elided]", gap))
				gap = 0
			}
			out = append(out, l)
			kept++
			continue
		}
		gap++
	}
	if gap > 0 {
		out = append(out, fmt.Sprintf("[%d lines elided]", gap))
	}
	out = append(out, lines[tailStart:]...)
	return out, true
}
