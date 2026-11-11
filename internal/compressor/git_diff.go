// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
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
	reGitDiffHeader = lazyre.New(`(?m)^diff --git a/.+ b/.+$`)
	// "index 1a2b3c4..5d6e7f8 100644" — pure provenance, safe to drop.
	reGitDiffIndex = lazyre.New(`^index [0-9a-f]+\.\.[0-9a-f]+(?: \d+)?$`)
	// Hunk header: "@@ -12,7 +12,6 @@ optional context".
	reGitDiffHunk = lazyre.New(`(?m)^@@ -\d+(?:,\d+)? \+\d+(?:,\d+)? @@`)
	// A git show transcript opens with a 40-hex commit line.
	reGitShowCommit = lazyre.New(`^commit [0-9a-f]{7,40}\b`)
	// Lines that carry a secret and must never be summarised away even when
	// they sit in a stretch of unchanged context: a diff can show a key being
	// moved or printed. If one appears anywhere, the whole input is preserved.
	reGitDiffSecret = lazyre.New(`(?i)(secret|token|password|passwd|api[_-]?key|private[_-]?key|access[_-]?key|BEGIN [A-Z ]*PRIVATE KEY|aws_secret|bearer )`)
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
// It returns (reason, false) when the input must be passed through unchanged.
//
// The compressors only ever drop unchanged context lines and the "index"
// provenance line; every change line, header, hunk header and commit line is
// kept verbatim (changeLinesPreserved verifies it). So a danger word
// (error/fail/panic/warning/...) on a line that is kept cannot be hidden, and
// it does not stop compaction: in a code diff such words are everywhere (an
// added `return fmt.Errorf(...)`, a file named errors.go, a commit message
// "fix failing test"). Two things still keep the whole input raw:
//   - a danger word on a stray line that is not part of the diff grammar (a
//     "warning:" or "fatal:" line git printed around the diff), because the
//     output is then about a failure, not only a diff;
//   - a secret-looking line among the context lines the compressor may drop,
//     so a key that sits in unchanged code is never summarised away.
//
// Owner decision, 2026-11-04: context lines are exempt from the danger gate.
// Widened 2026-11-10 to every line the compressor keeps verbatim, after real
// use showed 6 MB of git show and 1.2 MB of git diff kept raw by danger words
// on change lines and headers.
func gitDiffUnsafe(input []byte) (string, bool) {
	if diffStrayDanger(input) {
		return "git_diff_danger_word_preserved", false
	}
	if diffDroppableSecret(input) {
		return "git_diff_secret_preserved", false
	}
	return "", true
}

// reGitDiffHunkCounts reads the old and new line counts of a hunk header; an
// omitted count means 1.
var reGitDiffHunkCounts = lazyre.New(`^@@ -\d+(?:,(\d+))? \+\d+(?:,(\d+))? @@`)

// reGitDiffFileMeta is a file-level line between "diff --git" and the first
// hunk: ---/+++ paths, modes, renames, copies, similarity, binary notes.
var reGitDiffFileMeta = lazyre.New(`^(--- |\+\+\+ |old mode |new mode |deleted file mode |new file mode |similarity index |dissimilarity index |rename from |rename to |copy from |copy to |Binary files )`)

// diffLines walks a diff and calls visit for every line with its role:
// "preamble" (before the first file header), "grammar" (headers, hunk
// headers, change lines), "context" (an unchanged line inside a hunk) or
// "stray" (anything else). Hunk headers declare how many old and new lines
// the body holds, so a line after the body is used up is stray.
func diffLines(input []byte, visit func(line, role string)) {
	oldLeft, newLeft := 0, 0
	inDiff := false
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		role := "stray"
		switch m := reGitDiffHunkCounts.FindStringSubmatch(line); {
		case reGitDiffHeader.MatchString(line):
			inDiff, oldLeft, newLeft, role = true, 0, 0, "grammar"
		case !inDiff:
			role = "preamble"
		case m != nil:
			oldLeft, newLeft, role = hunkCount(m[1]), hunkCount(m[2]), "grammar"
		case oldLeft > 0 && newLeft > 0 && (strings.HasPrefix(line, " ") || line == ""):
			oldLeft, newLeft, role = oldLeft-1, newLeft-1, "context"
		case oldLeft > 0 && strings.HasPrefix(line, "-"):
			oldLeft, role = oldLeft-1, "grammar"
		case newLeft > 0 && strings.HasPrefix(line, "+"):
			newLeft, role = newLeft-1, "grammar"
		case strings.HasPrefix(line, `\ `), reGitDiffIndex.MatchString(line), reGitDiffFileMeta.MatchString(line):
			role = "grammar"
		}
		visit(line, role)
	}
}

// diffStrayDanger reports whether a danger word sits on a stray line, or on a
// preamble line of a bare diff (a git show preamble is its commit block).
func diffStrayDanger(input []byte) bool {
	show := isGitShow(input)
	found := false
	diffLines(input, func(line, role string) {
		if role == "stray" || (role == "preamble" && !show) {
			found = found || reDangerWord.MatchString(line)
		}
	})
	return found
}

// reGitDiffSecretValue is a context line that holds a secret, not one that
// only names the idea: a secret-named key given a value (`token = x`,
// `"api_key": "x"`), a private key block, an AWS secret or a bearer token.
// Prose and identifiers such as "tokens saved" or TokenCount do not match.
var reGitDiffSecretValue = lazyre.New(`(?i)((secret|token|password|passwd|api[_-]?key|private[_-]?key|access[_-]?key)["']?\s*(:=|=|:)|BEGIN [A-Z ]*PRIVATE KEY|aws_secret|bearer [A-Za-z0-9._~+/-]{8,})`)

// diffDroppableSecret reports whether a secret sits on a line the compressor
// may drop: an unchanged context line.
func diffDroppableSecret(input []byte) bool {
	found := false
	diffLines(input, func(line, role string) {
		if role == "context" {
			found = found || reGitDiffSecretValue.MatchString(line)
		}
	})
	return found
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
