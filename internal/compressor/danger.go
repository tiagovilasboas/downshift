// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
)

// reDangerWord matches the words that make a line unsafe to compact, wherever
// they appear: in a status line, a test title or a directory name.
var reDangerWord = regexp.MustCompile(`(?i)error|fail|panic|warning|exception|fatal`)

// linesHaveDangerWord reports whether any line of input carries a danger word.
// Lines matching allowed are skipped; use it only for a known, safe summary.
func linesHaveDangerWord(input []byte, allowed *regexp.Regexp) bool {
	for _, line := range bytes.Split(input, []byte("\n")) {
		if allowed != nil && allowed.Match(line) {
			continue
		}
		if reDangerWord.Match(line) {
			return true
		}
	}
	return false
}
