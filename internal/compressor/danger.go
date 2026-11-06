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

// reKeepLine is reDangerWord plus "warn": a line of console output a folded
// clean run may drop stays when it matches (a warn line is information).
var reKeepLine = regexp.MustCompile(`(?i)warn|did not exit|force exiting|open handle|obsolete|×|✕|✗|❯|` + reDangerWord.String())

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

// maxVerbatimConsole is how many unknown console lines a passing JS run keeps
// verbatim; above it they are dropped and counted.
const maxVerbatimConsole = 20

// consoleLine is one console line of a passing run: keep marks a line that
// always stays (a warning, an error word).
type consoleLine struct {
	text string
	keep bool
}

// foldConsole keeps every line, in order, when 20 or fewer are droppable; above
// that it keeps only the marked ones and counts the rest.
func foldConsole(lines []consoleLine) (kept []string, dropped int) {
	for _, l := range lines {
		if !l.keep {
			dropped++
		}
	}
	verbatim := dropped <= maxVerbatimConsole
	if verbatim {
		dropped = 0
	}
	for _, l := range lines {
		if l.keep || verbatim {
			kept = append(kept, l.text)
		}
	}
	return kept, dropped
}
