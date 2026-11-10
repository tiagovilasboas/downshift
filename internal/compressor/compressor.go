// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"unicode/utf8"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// MaxProcessableBytes prevents unbounded memory usage.
const MaxProcessableBytes = 2 * 1024 * 1024 // 2 MB

// Compress evaluates the input according to the policy and selected mode.
// If mode is ModeOff or ModeObserve, the returned Result.Output is byte-identical
// to input. In ModeSafe, recognized safe formats are compacted.
// Compress is CompressExit with an unknown exit code (-1). Text heuristics
// still fail open on error-shaped output, but a caller that knows the
// process exit code must use CompressExit.
func Compress(input []byte, mode Mode) Result {
	return CompressExit(input, mode, -1)
}

// CompressExit preserves the raw output when exitCode is non-zero.
// exitCode < 0 means the caller does not know the status.
func CompressExit(input []byte, mode Mode, exitCode int) Result {
	origLen := len(input)
	res := Result{
		Output:        input,
		Applied:       false,
		Format:        FormatUnknown,
		OriginalBytes: origLen,
		ReducedBytes:  origLen,
	}

	if exitCode > 0 {
		res.Reason = "nonzero_exit_preserved"
		return res
	}

	if origLen == 0 {
		res.Reason = "empty_input"
		return res
	}

	if mode == ModeOff {
		res.Reason = "compression_off"
		return res
	}

	// Memory safety guard: refuse inputs exceeding max bound
	if origLen > MaxProcessableBytes {
		res.Reason = "input_exceeds_memory_bound"
		return res
	}

	// Binary check: do not touch binary data
	if bytes.IndexByte(input, 0) != -1 || !utf8.Valid(input) {
		res.Reason = "binary_or_invalid_utf8"
		return res
	}

	// Detection and compaction read the text (colour.go); a raw answer is
	// always the original bytes.
	text, reason := screenText(input)
	collapsed := false
	if reason == "carriage_return_preserved" {
		// Progress frames collapse only when a terminal would show the same
		// text (progress.go); otherwise the bytes stay raw.
		if text, collapsed = collapseProgressText(input); collapsed {
			reason = ""
		}
	}
	if reason != "" {
		res.Reason = reason
		return res
	}

	// A recursive workspace run is read behind its prefix and named in a header.
	workspace := ""
	if !collapsed {
		text, workspace = stripWorkspacePrefix(text)
	}

	// Detect format
	format, isError := detectFormat(text)
	res.Format = format

	// Safety Rule: Fail-open on error.
	// Never summarize or alter errors, stack traces, or failing test suites.
	// A known zero exit means the runner passed whatever its console printed
	// ("Error:" in a log of a passing test); these compressors then check the
	// pass summary themselves and keep every danger line they cannot fold.
	if isError && !(exitCode == 0 && cleanExitTrusted(format, text)) {
		res.Reason = "error_detected_fail_open_raw_preserved"
		return res
	}

	// Attempt format-specific compression; text of no known shape goes through
	// the generic chain (generic.go), which keeps any danger word raw.
	var compacted []byte
	var applied bool
	if format == FormatUnknown && exitCode != 0 {
		// The generic chain needs a known clean exit; -1 is unknown.
		reason = "unrecognized_format"
	} else {
		compacted, applied, reason = applyFormatCompression(text, format)
	}
	if applied && format == FormatUnknown {
		res.Format = FormatGeneric
	}
	if !applied && collapsed && len(text) < len(input) {
		compacted, applied, reason = text, true, "progress frames collapsed"
		res.Format = FormatProgress
	}
	if applied && workspace != "" {
		compacted = append([]byte("Workspace: "+workspace+"\n"), compacted...)
		applied = len(compacted) < len(input)
		if !applied {
			reason = "no_compression_gain"
		}
	}
	if !applied {
		res.Reason = reason
		return res
	}

	res.Applied = true
	res.ReducedBytes = len(compacted)
	res.Reason = reason

	if mode == ModeSafe {
		res.Output = compacted
	} else {
		// ModeObserve: calculate reductions but preserve original output untouched
		res.Reason = fmt.Sprintf("observe_mode (potential %s)", reason)
	}

	return res
}

// reHardFailure is a failure the runner itself reports: a FAIL header, a
// failed test, a panic or a summary that counts failures. Console noise such
// as "Error:" in a log is not one.
var reHardFailure = lazyre.New(`(?m)^\s*(?:FAIL\b|--- FAIL|panic:|(?:Test Files|Tests|Test Suites):.*\bfailed\b)|^\s*FAIL\s`)

// cleanExitTrusted reports whether a zero exit overrides the error heuristic
// of detectFormat for format: the test runners whose compressor demands a
// complete pass summary and keeps each danger line verbatim, on text that
// carries no failure the runner reported.
func cleanExitTrusted(format Format, text []byte) bool {
	return (format == FormatGoTest || format == FormatVitest || format == FormatJest) &&
		!reHardFailure.Match(text)
}
