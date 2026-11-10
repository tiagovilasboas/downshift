// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// reGrepMatch reads one `grep -rn` row: the file, the line number and the match
// text. The file is the text before the first colon. A context row, a
// separator, a binary-file notice or a row without a file name does not match.
var reGrepMatch = lazyre.New(`^([^:]+):([0-9]+):(.*)$`)

// compressGrep groups the matches under their file, in the order grep found the
// files. Every line number and every match survives up to the caps in
// grep_caps.go, which count what they cut; only the leading whitespace of a
// match is dropped. A row that does not read as file:line:text keeps the
// output raw.
func compressGrep(input []byte) ([]byte, bool, string) {
	g := newMatchGroups()
	for _, line := range commandLines(input) {
		m := reGrepMatch.FindStringSubmatch(line)
		if m == nil {
			return input, false, "grep_unrecognized_structure_preserved"
		}
		g.add(m[1], m[2]+": "+strings.TrimLeft(m[3], " \t"))
	}
	out := []byte(g.render())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "grep matches grouped by file"
}
