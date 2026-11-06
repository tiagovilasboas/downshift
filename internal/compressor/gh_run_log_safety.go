// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// keepDangerLine appends msg to notes when it carries a danger word, so a
// warning, error or failure survives the gh run log summary wherever it
// appears, including inside a setup step the summary otherwise drops. These
// kept lines are notes, not a failure signal: a benign danger word (a git
// hint, a tar flag, a file name) must not flip a green job to failed. Real
// failures are captured separately (##[error] annotations, test/build
// failures, a non-zero step exit) and those drive the job status. The
// annotation lines (##[error], ##[warning]) are captured by their own path.
// Lines are de-duplicated and overlong ones truncated.
func keepDangerLine(notes *[]string, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" || strings.HasPrefix(msg, "##[") || !reDangerWord.MatchString(msg) {
		return
	}
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	for _, kept := range *notes {
		if kept == msg {
			return
		}
	}
	*notes = append(*notes, msg)
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
