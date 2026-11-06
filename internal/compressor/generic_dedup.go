// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "fmt"

// minDedupRun is how many identical adjacent lines collapse into one.
const minDedupRun = 3

// dedupLines collapses runs of minDedupRun or more identical adjacent
// non-blank lines into "line [xN]". It reports whether any run was folded.
func dedupLines(lines []string) ([]string, bool) {
	out := make([]string, 0, len(lines))
	folded := false
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && lines[j] == lines[i] {
			j++
		}
		if n := j - i; n >= minDedupRun && lines[i] != "" {
			out = append(out, fmt.Sprintf("%s [x%d]", lines[i], n))
			folded = true
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	return out, folded
}
