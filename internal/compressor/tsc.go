// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
)

var (
	reTSCPass  = regexp.MustCompile(`(?m)Found 0 errors`)
	reTSCError = regexp.MustCompile(`(?m)(?:error\s+TS\d+:|Found\s+[1-9]\d*\s+errors?)`)
)

var (
	// reTSCPlainLine matches the only lines a clean tsc watch-mode run prints.
	// A project path must end in tsconfig*.json, so no free-text word (error,
	// panic, FAIL) can ride along in a path. Any other line, including an
	// injected danger word or an ANSI-wrapped line, makes the transcript raw.
	reTSCPlainLine = regexp.MustCompile(`^(?:|\[\d{2}:\d{2}:\d{2}\] (?:Projects in this build: ?|Building project(?: '/?(?:[\w@.-]+/)*tsconfig(?:\.(?:app|node|build|base|spec))?\.json')?\.\.\.|Found 0 errors\. Watching for file changes\.)|    \* (?:[\w@.-]+/)*tsconfig(?:\.(?:app|node|build|base|spec))?\.json)$`)
	// reTSCSuccessLine is the 0-errors summary a clean run must end with.
	reTSCSuccessLine = regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\] Found 0 errors\. Watching for file changes\.$`)
)

func isTSC(input []byte) bool {
	return reTSCPass.Match(input) || reTSCError.Match(input) || bytes.Contains(input, []byte("error TS"))
}

// compressTSC compacts clean tsc output to a one-line summary:
// TSC: PASS (0 errors)
// If not strictly shorter than raw input, the input is returned.
func compressTSC(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, reTSCSuccessLine) {
		return input, false, "tsc_danger_word_preserved"
	}
	if !tscLinesRecognized(input) {
		return input, false, "tsc_unrecognized_line_preserved"
	}
	out := "TSC: PASS (0 errors)\n"
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "compressed tsc summary (0 errors)"
}

// tscLinesRecognized reports whether every line of input is part of the plain
// tsc success grammar and the run ends with its 0-errors summary line. A single
// unrecognised line makes the transcript unsafe to compact.
func tscLinesRecognized(input []byte) bool {
	summary := false
	for _, line := range bytes.Split(input, []byte("\n")) {
		if !reTSCPlainLine.Match(line) {
			return false
		}
		if reTSCSuccessLine.Match(line) {
			summary = true
		}
	}
	return summary
}
