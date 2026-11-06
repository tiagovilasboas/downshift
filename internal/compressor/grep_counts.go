// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// reGrepCount reads one `grep -c` / `rg -c` row: the file and its match count.
// A lone number (one file operand prints no path) does not match.
var reGrepCount = regexp.MustCompile(`^(.+):([0-9]+)$`)

// compressGrepCounts keeps the rows that have matches, in order, and folds the
// files with a count of 0 (grep -c lists every file it read; rg -c lists only
// files with matches) into one count. Past grepTotalCap rows the rest are cut
// and their number of files and matches is counted. A row that is not
// path:count keeps the output raw.
func compressGrepCounts(input []byte) ([]byte, bool, string) {
	var b strings.Builder
	shown, zero, cutFiles, cutMatches := 0, 0, 0, 0
	for _, line := range commandLines(input) {
		m := reGrepCount.FindStringSubmatch(line)
		if m == nil {
			return input, false, "grep_unrecognized_structure_preserved"
		}
		n, err := strconv.Atoi(m[2])
		switch {
		case err != nil:
			return input, false, "grep_unrecognized_structure_preserved"
		case n == 0:
			zero++
		case shown < grepTotalCap:
			b.WriteString(line + "\n")
			shown++
		default:
			cutFiles++
			cutMatches += n
		}
	}
	if zero > 0 {
		fmt.Fprintf(&b, "... %d files with 0 matches folded\n", zero)
	}
	if cutFiles > 0 {
		fmt.Fprintf(&b, "... %d more files with matches (%d matches) cut; narrow the search\n", cutFiles, cutMatches)
	}
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "grep counts, zero rows folded"
}
