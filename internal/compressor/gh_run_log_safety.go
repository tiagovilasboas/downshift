// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// keepDangerLine appends msg to important when it carries a danger word, so a
// failure, error or warning survives the gh run log summary wherever it
// appears, including inside a setup step the summary otherwise drops. The
// annotation lines (##[error], ##[warning]) are captured separately with their
// own de-duplication, so they are left to that path. Lines are de-duplicated
// and overlong ones truncated, matching the test/build-failure capture.
func keepDangerLine(important *[]string, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" || strings.HasPrefix(msg, "##[") || !reDangerWord.MatchString(msg) {
		return
	}
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	for _, kept := range *important {
		if kept == msg {
			return
		}
	}
	*important = append(*important, msg)
}

// hasUnparsedDangerLine reports whether input has a danger-word line that is
// not a job\tstep\tmessage row. Row danger lines are captured by
// keepDangerLine and survive the summary; a non-row danger line (a bare line
// the parser skips) would be lost, so its presence makes the whole log fail
// open and stay raw.
func hasUnparsedDangerLine(input []byte) bool {
	for _, line := range strings.Split(string(input), "\n") {
		if line == "" || reGhRunLogLine.MatchString(line) {
			continue
		}
		if reDangerWord.MatchString(line) {
			return true
		}
	}
	return false
}
