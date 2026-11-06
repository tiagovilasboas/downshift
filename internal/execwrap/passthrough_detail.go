// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "strings"

// Passthrough details: the closed set of codes that say why an
// unrecognized_output run stayed raw. Each is a constant, so the log holds no
// output text and no path. The reason a run passed through (Passthrough*)
// stays as it was; the detail only refines PassthroughUnrecognized.
const (
	// DetailDangerWord: the output holds a failure or secret word the
	// compressor keeps raw (error, panic, token, ...).
	DetailDangerWord = "danger_word"
	// DetailANSI: the output holds a terminal escape other than colour.
	DetailANSI = "ansi"
	// DetailCR: the output holds a carriage return (progress redraw).
	DetailCR = "cr"
	// DetailNoFinalNewline: the output does not end in a newline.
	DetailNoFinalNewline = "no_final_newline"
	// DetailUnknownShape: the compressor read the output and found no line
	// shape of the family's format it could summarise (this includes output
	// detected as a format the family may not be compacted as).
	DetailUnknownShape = "unknown_shape"
	// DetailNoCompressor: the family has no compressor at all.
	DetailNoCompressor = "no_compressor"
	// DetailCompressorPanic: the compressor panicked; the raw output was kept.
	DetailCompressorPanic = "compressor_panic"
)

// PassthroughDetails lists every detail code, for tests and the docs.
var PassthroughDetails = []string{
	DetailDangerWord, DetailANSI, DetailCR, DetailNoFinalNewline,
	DetailUnknownShape, DetailNoCompressor, DetailCompressorPanic,
}

// detailFromReason maps a compressor Result.Reason to a detail code. The
// compressor reasons are fixed strings, never output text.
func detailFromReason(reason string) string {
	switch {
	case reason == "terminal_escape_preserved":
		return DetailANSI
	case reason == "carriage_return_preserved":
		return DetailCR
	case reason == "no_final_newline_preserved":
		return DetailNoFinalNewline
	case reason == "error_detected_fail_open_raw_preserved",
		strings.Contains(reason, "danger"),
		strings.Contains(reason, "secret"):
		return DetailDangerWord
	}
	return DetailUnknownShape
}
