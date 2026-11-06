// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// The folded view of a long source file (cat, head, tail, sed -n of one file).
const (
	// FileReadMinLines is the shortest output that is folded. See
	// docs/reference/supported-tools.md ("Reading files") for the data behind it.
	FileReadMinLines = 300
	fileReadHead     = 8   // first lines kept
	fileReadTail     = 4   // last lines kept
	fileReadMinDecl  = 3   // fewer declarations: the file has no outline to keep
	fileReadMaxDecl  = 150 // declarations kept; the rest fold with their gap
	fileReadMaxLine  = 200 // bytes kept of one line
	fileReadMinGap   = 3   // a gap shorter than this is kept, not folded
	fileReadMaxRatio = 70  // the view must be under this percent of the input
)

// compressFileRead folds the body of a long source file to an outline: the
// first and last lines and every declaration line, each with its line number
// in the file, and a "[... N lines folded]" line for each gap. A closing note
// says how many lines were folded and the command that prints the whole text.
// It stays raw for output that is short, not a source file by its extension
// (h.Name), without at least fileReadMinDecl declarations, or not enough
// shorter. A line that holds a danger word (error, fail, panic, warning, ...)
// is always kept whole, like every other compressor keeps them. The words in
// the other kept lines are never changed except to cut a line past
// fileReadMaxLine bytes, which ends in "...".
func compressFileRead(input []byte, h CommandHint) ([]byte, bool, string) {
	lines := commandLines(input)
	total := len(lines)
	ext := fileExt(h.Name)
	switch {
	case ext == ".log":
		return compressLogRead(input, h)
	case h.JQ || ext == ".json":
		return compressJSONSchema(input, h)
	case total < FileReadMinLines:
		return input, false, "no_compression_gain"
	case !sourceExts[ext]:
		return input, false, "file_read_not_source_preserved"
	}
	keep := keptLines(lines, isMarkdown(ext))
	if keep == nil {
		return input, false, "file_read_no_outline_preserved"
	}
	first := max(h.First, 1)
	var b strings.Builder
	folded := 0
	for i := 0; i < total; {
		if keep[i] {
			fmt.Fprintf(&b, "%d: %s\n", first+i, cutLine(lines[i]))
			i++
			continue
		}
		j := i
		for j < total && !keep[j] {
			j++
		}
		fmt.Fprintf(&b, "[... %d lines folded]\n", j-i)
		folded += j - i
		i = j
	}
	fmt.Fprintf(&b, "[downshift read: %d of %d lines folded; full text: %s]\n", folded, total, fullTextCommand(h, first, total))
	out := []byte(b.String())
	if len(out)*100 >= len(input)*fileReadMaxRatio {
		return input, false, "no_compression_gain"
	}
	return out, true, "source file folded to its outline"
}

// keptLines marks the lines the view keeps, or returns nil when the file has
// fewer than fileReadMinDecl declarations. A gap shorter than fileReadMinGap
// is kept whole: a fold line would cost as much as the lines it hides.
func keptLines(lines []string, markdown bool) []bool {
	keep := make([]bool, len(lines))
	decls := 0
	for i, l := range lines {
		if isDeclLine(l, markdown) {
			decls++
			keep[i] = decls <= fileReadMaxDecl
		}
		keep[i] = keep[i] || reDangerWord.MatchString(l)
	}
	if decls < fileReadMinDecl {
		return nil
	}
	for i := 0; i < len(lines); i++ {
		keep[i] = keep[i] || i < fileReadHead || i >= len(lines)-fileReadTail
	}
	for i := 0; i < len(lines); {
		if keep[i] {
			i++
			continue
		}
		j := i
		for j < len(lines) && !keep[j] {
			j++
		}
		if j-i < fileReadMinGap && i > 0 && j < len(lines) {
			for k := i; k < j; k++ {
				keep[k] = true
			}
		}
		i = j
	}
	return keep
}

// fullTextCommand is the command that prints every line the view folded: the
// exact sed range of the lines read, or the tail command for a tail.
func fullTextCommand(h CommandHint, first, total int) string {
	name := h.Name
	if name == "" {
		name = "<file>"
	}
	if h.Tail {
		return fmt.Sprintf("tail -n %d %s (numbers count from the first line shown)", total, name)
	}
	return fmt.Sprintf("sed -n '%d,%dp' %s", first, first+total-1, name)
}

// cutLine cuts a line past fileReadMaxLine bytes at a rune boundary, unless it
// holds a danger word.
func cutLine(s string) string {
	if len(s) <= fileReadMaxLine || reDangerWord.MatchString(s) {
		return s
	}
	n := fileReadMaxLine
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n] + "..."
}
