// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
)

// reMakeFailure is the line make prints when a recipe fails: the failing
// target, its exit status, and the "Stop" that ends the run.
var reMakeFailure = regexp.MustCompile(`(?m)^make(?:\[\d+\])?: \*\*\* .*(?:Error \d+|Stop\.?)\s*$`)

// plainMakeFailedText accepts a failed make transcript only as written: no
// CRLF or missing final newline, and a make failure line in it. Without that
// line the run is not one this compactor knows, so it stays raw.
func plainMakeFailedText(input []byte) ([]byte, bool) {
	if input[len(input)-1] != '\n' || bytes.IndexByte(input, '\r') != -1 || !reMakeFailure.Match(input) {
		return nil, false
	}
	return input, true
}

// compactMakeFailedRun cuts the passing noise of the go test run a failed
// make target contains (ok package lines, passed tests and their run markers)
// and keeps every other line byte for byte: the failing target line, the
// first error block, each "make: *** [...] Error N" line. A transcript with
// no go test failure in it has nothing this compactor can prove is noise, so
// it stays raw.
func compactMakeFailedRun(text string) (string, string) {
	if !reGoFailedSignal.MatchString(text) {
		return "", "make_failed_run_no_gain"
	}
	out, ok := compactFailedGoTest(text)
	if !ok {
		return "", "make_failed_run_no_gain"
	}
	return out, ""
}
