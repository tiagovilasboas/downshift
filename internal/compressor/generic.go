// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// FormatGeneric marks output that matched no known tool shape and was
// compacted by the generic chain: base64 elision, a danger gate, dedup of
// repeated lines, then a head/tail cap. It is never detected from the text;
// CompressExit reports it only when the chain actually shortened the output.
const FormatGeneric Format = "generic"

// genericNoGain is the reason when the chain ran but the result is not shorter.
const genericNoGain = "no_compression_gain"

// reGenericDanger widens reDangerWord for text of an unknown shape. The chain
// cuts lines it cannot read, so a failure marker, a stack frame or a refusal
// anywhere in the output keeps the whole run raw.
var reGenericDanger = lazyre.New(
	`(?i)not ok\b|\berr\b|^\s+at .+:\d+`)

// dangerSubstrings are checked on the lower-cased text with strings.Contains,
// which stays fast on a multi-megabyte line.
var dangerSubstrings = []string{
	"error", "fail", "panic", "warn", "exception", "fatal", "denied", "refused",
	"timeout", "timed out", "time out", "cannot", "can't", "couldn't", "could not",
	"unable", "abort", "killed", "segfault", "traceback", "assert", "invalid",
	"unexpected", "✗", "✘", "✖", "❌", "err!", "errno", "enoent", "eacces",
	"not found", "no such", "deprecat", "critical", "rejected", "conflict",
	"exit status", "exited with", "stack",
}

// maxRegexLine bounds the lines the regexps read; longer lines (a blob) only
// get the substring check.
const maxRegexLine = 4096

// reChainMarker matches text that already looks like a chain marker; such
// input is kept raw so a marker is never ambiguous.
var reChainMarker = lazyre.New(`(?:lines|frames) elided\]|\[base64 |\[x[0-9]+\]$`)

// compressGeneric runs the generic chain over text of an unknown shape. It
// returns the input untouched, with a reason, when the text carries any
// danger word or when no step shortened it.
func compressGeneric(input []byte) ([]byte, bool, string) {
	// The gate reads the original text: a signal inside or beside a long run
	// must not be elided before it is seen.
	// Only the frames of a foldable stack run (generic_stack.go) are left out.
	all := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	gated, stacked, stackFolded := foldStackRuns(all)
	gateText := string(input)
	if stackFolded {
		gateText = strings.Join(gated, "\n")
	}
	if genericHasDanger(gateText) || reChainMarker.Match(input) {
		return input, false, "generic_danger_word_kept_raw"
	}
	text, blobs := elideBase64(strings.Join(stacked, "\n") + "\n")
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	lines, folded := dedupLines(lines)
	lines, capped := capLines(lines)
	if blobs == 0 && !folded && !capped && !stackFolded {
		return input, false, "unrecognized_format"
	}
	out := strings.Join(lines, "\n") + "\n"
	if len(out) >= len(input) {
		return input, false, genericNoGain
	}
	return []byte(out), true, "generic output compacted"
}

// genericHasDanger reports whether any line is unsafe to cut.
func genericHasDanger(text string) bool {
	lower := strings.ToLower(text)
	for _, d := range dangerSubstrings {
		if strings.Contains(lower, d) {
			return true
		}
	}
	for _, line := range strings.Split(text, "\n") {
		if len(line) > maxRegexLine {
			continue
		}
		b := []byte(line)
		if reChainMarker.Match(b) || reGenericDanger.Match(b) {
			return true
		}
	}
	return false
}
