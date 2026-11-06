// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "strings"

// contextLines is the number of unchanged context lines kept on each side
// of a run of changes (git's default is 3).
const contextLines = 2

// rewriteUnifiedDiff walks a unified diff (optionally preceded by a git show
// commit/metadata block, kept verbatim when keepPreamble is true) and emits a
// compacted version. It returns (output, fileCount, recognized). recognized is
// false when the input contains no "diff --git" file section at all.
func rewriteUnifiedDiff(input []byte, keepPreamble bool) ([]byte, int, bool) {
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")

	var b strings.Builder
	files := 0
	inDiff := false   // seen the first "diff --git" header
	var hunk []string // buffered lines of the current hunk body
	haveHunk := false // a hunk header has been emitted and body is buffering

	flushHunk := func() {
		if !haveHunk {
			return
		}
		for _, l := range trimContext(hunk) {
			b.WriteString(l)
			b.WriteByte('\n')
		}
		hunk = hunk[:0]
		haveHunk = false
	}

	for _, line := range lines {
		switch {
		case reGitDiffHeader.MatchString(line):
			flushHunk()
			files++
			inDiff = true
			b.WriteString(line)
			b.WriteByte('\n')
		case !inDiff:
			// Preamble before the first file section. For git show this is the
			// commit/metadata/message block; keep it. For a bare diff there is
			// nothing meaningful here, so drop it.
			if keepPreamble {
				b.WriteString(line)
				b.WriteByte('\n')
			}
		case reGitDiffIndex.MatchString(line):
			// Pure provenance; drop.
		case reGitDiffHunk.MatchString(line):
			flushHunk()
			b.WriteString(line)
			b.WriteByte('\n')
			haveHunk = true
		case haveHunk:
			hunk = append(hunk, line)
		default:
			// File-level metadata between the header and the first hunk:
			// "--- a/x", "+++ b/x", "new file mode ...", "rename from ...",
			// "Binary files ... differ". These carry structure an agent acts
			// on, so keep them verbatim.
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	flushHunk()

	if files == 0 {
		return nil, 0, false
	}
	return []byte(b.String()), files, true
}

// trimContext collapses leading/trailing/interior runs of unchanged context
// lines (those starting with a space, or the "\ No newline at end of file"
// marker) to at most contextLines, while keeping every +/- change line. A run
// of context longer than 2*contextLines between two changes is replaced by the
// first contextLines, an elision marker, and the last contextLines.
func trimContext(body []string) []string {
	isContext := func(l string) bool {
		return l == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, `\ `)
	}

	// Index of the first and last change line in the body.
	first, last := -1, -1
	for i, l := range body {
		if !isContext(l) {
			if first == -1 {
				first = i
			}
			last = i
		}
	}
	if first == -1 {
		// No change lines at all (should not happen in a real hunk); keep as-is.
		return body
	}

	var out []string

	// Leading context: keep at most contextLines immediately before first change.
	lead := body[:first]
	if len(lead) > contextLines {
		lead = lead[len(lead)-contextLines:]
	}
	out = append(out, lead...)

	// Middle: walk from first change to last change, collapsing interior
	// context runs that exceed 2*contextLines.
	mid := body[first : last+1]
	i := 0
	for i < len(mid) {
		if !isContext(mid[i]) {
			out = append(out, mid[i])
			i++
			continue
		}
		// Measure the run of context.
		j := i
		for j < len(mid) && isContext(mid[j]) {
			j++
		}
		run := mid[i:j]
		if len(run) > 2*contextLines {
			out = append(out, run[:contextLines]...)
			out = append(out, "...")
			out = append(out, run[len(run)-contextLines:]...)
		} else {
			out = append(out, run...)
		}
		i = j
	}

	// Trailing context: keep at most contextLines immediately after last change.
	trail := body[last+1:]
	if len(trail) > contextLines {
		trail = trail[:contextLines]
	}
	out = append(out, trail...)

	return out
}
