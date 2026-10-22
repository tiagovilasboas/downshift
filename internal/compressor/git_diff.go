// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// Git diff / show / stash list compressors.
//
// RTK principle: strip decoration that an agent never acts on, keep every
// change line and every hunk boundary. These compressors NEVER drop an added
// or removed line; they only remove noise (the "index <sha>..<sha> <mode>"
// line, the redundant "diff --git a/x b/x" header) and shrink unchanged
// context from the default 3 lines to 2 around each run of changes. Hunk
// headers (@@ ... @@) and file boundaries (---/+++) are always preserved so
// the diff stays apply-readable and the agent still sees where each change is.

var (
	// A unified diff starts with "diff --git a/<path> b/<path>".
	reGitDiffHeader = regexp.MustCompile(`(?m)^diff --git a/.+ b/.+$`)
	// "index 1a2b3c4..5d6e7f8 100644" — pure provenance, safe to drop.
	reGitDiffIndex = regexp.MustCompile(`^index [0-9a-f]+\.\.[0-9a-f]+(?: \d+)?$`)
	// Hunk header: "@@ -12,7 +12,6 @@ optional context".
	reGitDiffHunk = regexp.MustCompile(`(?m)^@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@`)
	// A git show transcript opens with a 40-hex commit line.
	reGitShowCommit = regexp.MustCompile(`^commit [0-9a-f]{7,40}\b`)
	// Lines that carry a secret and must never be summarised away even when
	// they sit in a stretch of unchanged context: a diff can show a key being
	// moved or printed. If one appears anywhere, the whole input is preserved.
	reGitDiffSecret = regexp.MustCompile(`(?i)(secret|token|password|passwd|api[_-]?key|private[_-]?key|access[_-]?key|BEGIN [A-Z ]*PRIVATE KEY|aws_secret|bearer )`)
)

// isGitDiff reports whether input is a bare unified diff: it opens with a
// "diff --git a/.. b/.." header and every file section carries at least one
// hunk. A transcript that opens with a commit line is a git show, not a diff.
func isGitDiff(input []byte) bool {
	s := string(input)
	if !strings.HasPrefix(s, "diff --git a/") {
		return false
	}
	return reGitDiffHunk.Match(input)
}

// isGitShow reports whether input is a git show transcript: a commit metadata
// block followed by a unified diff. The diff section is what distinguishes it
// from git log, which also opens with "commit <hash>" but carries no diff.
func isGitShow(input []byte) bool {
	if !reGitShowCommit.Match(input) {
		return false
	}
	return reGitDiffHeader.Match(input) && reGitDiffHunk.Match(input)
}

// contextLines is the number of unchanged context lines RTK keeps on each side
// of a run of changes (git's default is 3).
const contextLines = 2

// compressGitDiff strips index/diff-header decoration and trims unchanged
// context to contextLines around each change, preserving every +/- line, every
// hunk header and every file boundary. If the result is not strictly shorter,
// the raw input is returned.
func compressGitDiff(input []byte) ([]byte, bool, string) {
	if reason, ok := gitDiffUnsafe(input); !ok {
		return input, false, reason
	}
	out, files, ok := rewriteUnifiedDiff(input, false)
	if !ok {
		return input, false, "git_diff_unrecognized_structure_preserved"
	}
	if !changeLinesPreserved(input, out) {
		return input, false, "git_diff_change_line_dropped_preserved"
	}
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed git diff (%d files)", files)
}

// compressGitShow keeps the commit metadata block verbatim (hash, author,
// date, message) and applies the same diff trimming to the patch body.
func compressGitShow(input []byte) ([]byte, bool, string) {
	if reason, ok := gitDiffUnsafe(input); !ok {
		return input, false, reason
	}
	out, files, ok := rewriteUnifiedDiff(input, true)
	if !ok {
		return input, false, "git_show_unrecognized_structure_preserved"
	}
	if !changeLinesPreserved(input, out) {
		return input, false, "git_show_change_line_dropped_preserved"
	}
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed git show (%d files)", files)
}

// gitDiffUnsafe is the fail-open safety gate for the diff/show compressors.
// It returns (reason, false) when the input must be passed through unchanged:
// any danger word (error/fail/panic/warning/...) or any secret-looking line
// makes compaction unsafe, because trimming unchanged context could drop a
// line an agent needs to read. When it returns ("", true), compaction may
// proceed but is still verified line-by-line by changeLinesPreserved.
func gitDiffUnsafe(input []byte) (string, bool) {
	if linesHaveDangerWord(input, nil) {
		return "git_diff_danger_word_preserved", false
	}
	if reGitDiffSecret.Match(input) {
		return "git_diff_secret_preserved", false
	}
	return "", true
}

// changeLinesPreserved is the mutation-closure invariant: every added ("+"),
// removed ("-") or commit-metadata line of the raw diff must appear verbatim
// in the compacted output. Context lines (space-prefixed) and the dropped
// "index" provenance line are exempt; everything an agent acts on must
// survive. A single missing change line forces a byte-identical pass-through.
func changeLinesPreserved(input, out []byte) bool {
	have := map[string]int{}
	for _, l := range strings.Split(string(out), "\n") {
		have[l]++
	}
	for _, l := range strings.Split(string(input), "\n") {
		if l == "" || strings.HasPrefix(l, " ") || strings.HasPrefix(l, `\ `) {
			continue // unchanged context or "no newline" marker
		}
		if reGitDiffIndex.MatchString(l) {
			continue // provenance line the compressor is allowed to drop
		}
		if strings.HasPrefix(l, "diff --git a/") {
			continue // header the compressor keeps but may reformat per file
		}
		if have[l] > 0 {
			have[l]--
			continue
		}
		return false
	}
	return true
}

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

// --- git stash list ---------------------------------------------------------

// "stash@{0}: WIP on main: 1a2b3c4 commit subject" or
// "stash@{1}: On main: message".
var reGitStashEntry = regexp.MustCompile(`^stash@\{(\d+)\}: (.+)$`)

// isGitStashList reports whether every non-empty line is a stash entry.
func isGitStashList(input []byte) bool {
	any := false
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !reGitStashEntry.MatchString(line) {
			return false
		}
		any = true
	}
	return any
}

// compressGitStashList keeps one compact line per stash: "{index} <summary>",
// where the summary drops the redundant "WIP on <branch>:" / "On <branch>:"
// boilerplate but preserves the commit hint and subject.
func compressGitStashList(input []byte) ([]byte, bool, string) {
	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	var b strings.Builder
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := reGitStashEntry.FindStringSubmatch(line)
		if m == nil {
			return input, false, "git_stash_unrecognized_line_preserved"
		}
		idx, desc := m[1], strings.TrimSpace(m[2])
		// Drop the "WIP on <branch>: " / "On <branch>: " boilerplate prefix.
		if p := strings.Index(desc, ": "); p >= 0 {
			head := desc[:p]
			if strings.HasPrefix(head, "WIP on ") || strings.HasPrefix(head, "On ") {
				desc = strings.TrimSpace(desc[p+2:])
			}
		}
		fmt.Fprintf(&b, "{%s} %s\n", idx, desc)
		count++
	}
	if count == 0 {
		return input, false, "git_stash_empty"
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed git stash list (%d entries)", count)
}
