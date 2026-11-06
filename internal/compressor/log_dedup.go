// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// The deduplicated view of a log file read (cat x.log, tail -n N x.log).
const (
	// LogReadMinLines is the shortest log read that is deduplicated.
	LogReadMinLines = 40
	logMinRun       = 3  // a run shorter than this is kept whole
	logMaxRatio     = 70 // the view must be under this percent of the input
)

var (
	reLogUUID  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	reLogDigit = regexp.MustCompile(`[0-9]+`)
)

// logShape is a line with its volatile parts (ids and every run of digits,
// so timestamps and counters) blanked out. Two lines of one shape differ only
// in those.
func logShape(line string) string {
	return reLogDigit.ReplaceAllString(reLogUUID.ReplaceAllString(line, "<id>"), "#")
}

// compressLogRead collapses each run of three or more consecutive lines of
// one shape into its first line and a count, with the last line of the run
// when the lines differ. A run of identical lines collapses even when it holds
// a danger word; a run of lines that merely resemble each other and holds one
// is kept whole, since its numbers may be the point. Blank lines are kept.
// Output that is short, not shorter or has nothing to collapse stays raw.
func compressLogRead(input []byte, h CommandHint) ([]byte, bool, string) {
	lines := commandLines(input)
	if len(lines) < LogReadMinLines {
		return input, false, "no_compression_gain"
	}
	var b strings.Builder
	folded := 0
	for i := 0; i < len(lines); {
		j := i + 1
		shape := logShape(lines[i])
		for strings.TrimSpace(lines[i]) != "" && j < len(lines) && logShape(lines[j]) == shape {
			j++
		}
		if j-i >= logMinRun && collapsible(lines[i:j]) {
			b.WriteString(lines[i] + "\n" + foldNote(lines[i:j]) + "\n")
			folded += j - i - 1
		} else {
			b.WriteString(strings.Join(lines[i:j], "\n") + "\n")
		}
		i = j
	}
	if folded == 0 {
		return input, false, "log_no_repeats_preserved"
	}
	first := max(h.First, 1)
	fmt.Fprintf(&b, "[downshift read: %d of %d lines folded; full text: %s]\n", folded, len(lines), fullTextCommand(h, first, len(lines)))
	if b.Len()*100 >= len(input)*logMaxRatio {
		return input, false, "no_compression_gain"
	}
	return []byte(b.String()), true, "repeated log lines collapsed"
}

// collapsible reports whether a run may be cut: its lines are identical, or
// none of them holds a danger word.
func collapsible(run []string) bool {
	same := true
	for _, l := range run {
		same = same && l == run[0]
	}
	if same {
		return true
	}
	for _, l := range run {
		if reDangerWord.MatchString(l) {
			return false
		}
	}
	return true
}

// foldNote says how many lines of the run were cut after its first line.
func foldNote(run []string) string {
	last := run[len(run)-1]
	if last == run[0] {
		return fmt.Sprintf("[... %d identical lines folded]", len(run)-1)
	}
	return fmt.Sprintf("[... %d similar lines folded (timestamps and numbers differ); last: %s]", len(run)-1, cutLine(last))
}
