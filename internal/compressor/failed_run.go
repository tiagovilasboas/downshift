// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"unicode/utf8"
)

// failedRunFormat describes how one format compacts a run that exited
// non-zero. Every format keeps each failure line byte for byte and cuts only
// lines of tests that passed.
type failedRunFormat struct {
	// name prefixes the Reason of a run that stays raw.
	name string
	// label is the text after "compressed failed" in the Reason of a compacted run.
	label string
	// text reads the bytes the compactor parses. ok false keeps the run raw.
	text func(input []byte) ([]byte, bool)
	// compact returns the compacted text, or a non-empty reason when the run
	// must stay raw because a line is not one the format's grammar knows.
	compact func(text string) (out, reason string)
}

// failedRunFormats lists the formats CompressFailed supports.
var failedRunFormats = map[Format]failedRunFormat{
	FormatGoTest:     {name: "go_test", label: "go test", text: plainFailedGoTestText, compact: compactGoTestFailedRun},
	FormatVitest:     {name: "vitest", label: "vitest", text: colourFreeFailedText, compact: compactFailedVitest},
	FormatPlaywright: {name: "playwright", label: "playwright", text: colourFreeFailedText, compact: compactFailedPlaywright},
	FormatJest:       {name: "jest", label: "jest", text: colourFreeFailedText, compact: compactFailedJest},
	FormatPytest:     {name: "pytest", label: "pytest", text: colourFreeFailedText, compact: compactFailedPytest},
	FormatCargoTest:  {name: "cargo_test", label: "cargo test", text: plainFailedCargoText, compact: compactFailedCargo},
	// make wraps another program's transcript; FormatGoTestRuns names it.
	FormatGoTestRuns: {name: "make", label: "make", text: plainMakeFailedText, compact: compactMakeFailedRun},
}

// CompressFailed compacts the output of a command that exited non-zero, for
// the formats that support it (go test, vitest, playwright, jest, pytest and cargo test). It cuts only
// the lines of tests and packages that passed and keeps every other line byte
// for byte, so each failure keeps its context. Output it cannot classify,
// CRLF, a missing final newline, or output without a failure signal comes back
// unchanged with Applied false (fail open).
func CompressFailed(input []byte, want Format) Result {
	res := Result{Output: input, Format: FormatUnknown, OriginalBytes: len(input), ReducedBytes: len(input)}
	f, ok := failedRunFormats[want]
	if !ok {
		res.Reason = "failed_run_format_unsupported"
		return res
	}
	res.Format = want
	if len(input) == 0 || len(input) > MaxProcessableBytes ||
		bytes.IndexByte(input, 0) != -1 || !utf8.Valid(input) {
		res.Reason = f.name + "_failed_run_raw_preserved"
		return res
	}
	text, ok := f.text(input)
	if !ok {
		res.Reason = f.name + "_failed_run_raw_preserved"
		return res
	}
	out, reason := f.compact(string(text))
	if reason != "" {
		res.Reason = reason
		return res
	}
	if len(out) >= len(input) {
		res.Reason = f.name + "_failed_run_no_gain"
		return res
	}
	res.Output = []byte(out)
	res.ReducedBytes = len(out)
	res.Applied = true
	res.Reason = "compressed failed " + f.label + " run (failures kept verbatim)"
	return res
}

// plainFailedGoTestText accepts go test output only as it was written: no
// colour, CRLF or missing final newline, and or a `go test -json` stream without a failed action.
func plainFailedGoTestText(input []byte) ([]byte, bool) {
	if input[len(input)-1] != '\n' || bytes.IndexByte(input, '\r') != -1 {
		return nil, false
	}
	if isGoJSON(input) {
		return input, bytes.Contains(input, []byte(`"Action":"fail"`))
	}
	return input, !reGoFailedJSON.Match(input) && reGoFailedSignal.Match(input)
}

// colourFreeFailedText returns the text of a vitest or playwright run with its
// SGR colour removed (screenText), since both print colour even when stdout is
// not a terminal. Any other escape sequence, CRLF or a missing final newline
// keeps the run raw.
func colourFreeFailedText(input []byte) ([]byte, bool) {
	text, reason := screenText(input)
	return text, reason == ""
}

// compactGoTestFailedRun adapts compactFailedGoTest to the shared contract.
func compactGoTestFailedRun(text string) (string, string) {
	if isGoJSON([]byte(text)) {
		return compactFailedGoTestJSON(text)
	}
	out, ok := compactFailedGoTest(text)
	if !ok {
		return "", "go_test_failed_run_no_gain"
	}
	return out, ""
}
