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
// Principle: strip decoration that an agent never acts on, keep every
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
// a danger word (error/fail/panic/warning/...) on any line other than an
// unchanged context line, or any secret-looking line, makes compaction unsafe.
// A danger word on a +/- line, a header, a hunk header or a stray non-diff line
// (for example a "warning:" line mixed into the output) means the text is
// about a failure, and the whole input stays raw. A danger word on a context
// line (a leading single space inside a hunk, counted from the hunk header)
// is source code that did not change: trimming it cannot hide a failure, and
// every +/- line is verified to survive by changeLinesPreserved (owner
// decision, 2026-11-04). When it returns ("", true), compaction may proceed.
func gitDiffUnsafe(input []byte) (string, bool) {
	if diffDangerOutsideContext(input) {
		return "git_diff_danger_word_preserved", false
	}
	if reGitDiffSecret.Match(input) {
		return "git_diff_secret_preserved", false
	}
	return "", true
}

// reGitDiffHunkCounts reads the old and new line counts of a hunk header; an
// omitted count means 1.
var reGitDiffHunkCounts = regexp.MustCompile(`^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@`)

// diffDangerOutsideContext reports whether a danger word sits on any line that
// is not an unchanged context line of a hunk. The hunk header declares how many
// old and new lines the body holds, so a space-prefixed line counts as context
// only while those counts are not used up; a stray indented line after the
// hunk is outside it and is checked like any other.
func diffDangerOutsideContext(input []byte) bool {
	oldLeft, newLeft := 0, 0
	for _, line := range strings.Split(string(input), "\n") {
		if m := reGitDiffHunkCounts.FindStringSubmatch(line); m != nil {
			oldLeft, newLeft = hunkCount(m[1]), hunkCount(m[2])
		} else if oldLeft > 0 || newLeft > 0 {
			switch {
			case strings.HasPrefix(line, " "):
				if oldLeft > 0 && newLeft > 0 {
					oldLeft--
					newLeft--
					continue // unchanged context: exempt from the danger gate
				}
			case strings.HasPrefix(line, "-"):
				oldLeft--
			case strings.HasPrefix(line, "+"):
				newLeft--
			}
		}
		if reDangerWord.MatchString(line) {
			return true
		}
	}
	return false
}

// hunkCount parses a hunk header count; empty means the default of 1.
func hunkCount(s string) int {
	if s == "" {
		return 1
	}
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
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
