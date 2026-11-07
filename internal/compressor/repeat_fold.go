// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// Fold shape: a run is at least foldMinRun consecutive non-empty lines with the
// same shape. The shape of a line is the line with every digit run replaced by
// one "#", so `test_case_007 ... ok` and `test_case_008 ... ok` match while any
// other change (a different word, a different status) breaks the run. A run
// keeps its first foldKeepHead and last foldKeepTail lines and puts one marker
// between them. A run with any line that carries a danger word or a secret is
// left whole. This is lossy by design: callers use it only when the raw output
// is stored elsewhere (CCR) and can be retrieved.
//
// File paths: when a run consists of distinct file paths (e.g. from find or ls),
// folding down to 5 lines starves agents of needed paths. Such runs require at
// least foldPathMinRun lines to fold, and retain foldPathKeepHead and
// foldPathKeepTail lines instead of 3 and 2.
const (
	foldMinRun       = 8
	foldKeepHead     = 3
	foldKeepTail     = 2
	foldPathMinRun   = 60
	foldPathKeepHead = 20
	foldPathKeepTail = 10
)

var reDigitRun = regexp.MustCompile(`[0-9]+`)

// foldNoRun is the reason when no run of similar lines was found or kept.
const foldNoRun = "no_repeat_run"

func lineShape(line string) string { return reDigitRun.ReplaceAllString(line, "#") }

// foldLineUnsafe reports whether a line must never be folded away.
func foldLineUnsafe(line string) bool {
	return genericHasDanger(line) || reGitDiffSecret.MatchString(line)
}

// isFilePathLine reports whether a line looks like a file path or filename.
func isFilePathLine(s string) bool {
	if s == "" || strings.Contains(s, " ... ") || strings.Contains(s, ":") {
		return false
	}
	if strings.Contains(s, " ") || strings.Contains(s, "\t") {
		return false
	}
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return true
	}
	if i := strings.LastIndexByte(s, '.'); i > 0 && i < len(s)-1 {
		ext := s[i+1:]
		if len(ext) >= 1 && len(ext) <= 6 {
			return true
		}
	}
	return false
}

// isFilePathRun reports whether run is a sequence of distinct file path lines.
func isFilePathRun(run []string) bool {
	if len(run) == 0 {
		return false
	}
	seen := make(map[string]struct{}, len(run))
	for _, l := range run {
		if !isFilePathLine(l) {
			return false
		}
		if _, exists := seen[l]; exists {
			return false
		}
		seen[l] = struct{}{}
	}
	return true
}

// FoldRepeats folds runs of similarly shaped lines (see the constants above).
// Lines outside a run are kept byte for byte.
func FoldRepeats(input []byte) Result {
	res := Result{Output: input, Format: FormatGeneric, OriginalBytes: len(input), ReducedBytes: len(input), Reason: foldNoRun}
	lines := strings.Split(string(input), "\n")
	out := make([]string, 0, len(lines))
	folded := false
	for i := 0; i < len(lines); {
		j := i + 1
		if lines[i] != "" {
			shape := lineShape(lines[i])
			for j < len(lines) && lines[j] != "" && lineShape(lines[j]) == shape {
				j++
			}
		}
		run := lines[i:j]
		minRun := foldMinRun
		head := foldKeepHead
		tail := foldKeepTail
		if isFilePathRun(run) {
			minRun = foldPathMinRun
			head = foldPathKeepHead
			tail = foldPathKeepTail
		}
		if j-i >= minRun && j-i > head+tail && !runUnsafe(run) {
			out = append(out, lines[i:i+head]...)
			out = append(out, fmt.Sprintf("[... %d similar lines folded]", j-i-head-tail))
			out = append(out, lines[j-tail:j]...)
			folded = true
		} else {
			out = append(out, lines[i:j]...)
		}
		i = j
	}
	if !folded {
		return res
	}
	text := strings.Join(out, "\n")
	if len(text) >= len(input) {
		res.Reason = genericNoGain
		return res
	}
	res.Output, res.Applied, res.ReducedBytes, res.Reason = []byte(text), true, len(text), "similar lines folded"
	return res
}

func runUnsafe(run []string) bool {
	for _, l := range run {
		if foldLineUnsafe(l) {
			return true
		}
	}
	return false
}
