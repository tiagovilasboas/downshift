// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
	"strings"
)

// FormatMarkdown marks the markdown body that `gh issue view` and `gh release
// view` print after their header block. Only what is provably redundant is
// removed: blank-line runs, whole-line HTML comments, shields.io badge lines
// and a heading repeated right under itself. Code fences, indented code,
// tables and every link target outside a removed badge stay verbatim.
const FormatMarkdown Format = "markdown"

var (
	reMdHeaderLine = regexp.MustCompile(`^[a-z-]+:\t.*$`)
	reMdFence      = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
	reMdComment    = regexp.MustCompile(`^\s*<!--.*-->\s*$`)
	reMdHeading    = regexp.MustCompile(`^ {0,3}#{1,6}\s+\S`)
	reMdBadgeItem  = regexp.MustCompile(`\[?!\[[^\]]*\]\(https://(?:img\.shields\.io|badgen\.net|badge\.fury\.io|codecov\.io)/[^)\s]*\)(?:\]\([^)\s]*\))?`)
	reMdStackFrame = regexp.MustCompile(`^\s+at \S.*[(:]\S*:\d+|^Traceback \(most recent call last\)|^\s+File ".*", line \d+`)
)

// isGhMarkdownView reports a non-TTY `gh issue view` or `gh release view`:
// a header of "key:\tvalue" lines, a "--" separator and a markdown body. A
// pull request carries url and additions fields and is left to gh_pr.
func isGhMarkdownView(input []byte) bool {
	head, _, ok := splitGhBody(string(input))
	if !ok || !strings.HasPrefix(head[0], "title:\t") {
		return false
	}
	has := map[string]bool{}
	for _, l := range head {
		if i := strings.IndexByte(l, ':'); i > 0 {
			has[l[:i]] = true
		}
	}
	issue := has["comments"] && has["number"] && !has["url"] && !has["additions"]
	release := has["tag"] && !has["state"]
	return issue || release
}

// splitGhBody splits at the first "--" line into header lines and body lines.
func splitGhBody(s string) (head, body []string, ok bool) {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l == "--" {
			return lines[:i], lines[i+1:], i > 0
		}
	}
	return nil, nil, false
}

// compressMarkdownView slims the body of a gh issue or release view. Any line
// of the header outside "key:\tvalue", any danger word or stack frame, and an
// unclosed code fence keep the whole output raw.
func compressMarkdownView(input []byte) ([]byte, bool, string) {
	head, body, ok := splitGhBody(string(input))
	if !ok {
		return input, false, "markdown_no_body"
	}
	for _, l := range head {
		if !reMdHeaderLine.MatchString(l) {
			return input, false, "markdown_unknown_header_line_preserved"
		}
	}
	if linesHaveDangerWord(input, nil) {
		return input, false, "markdown_danger_word_preserved"
	}
	for _, l := range body {
		if reMdStackFrame.MatchString(l) {
			return input, false, "markdown_stack_frame_preserved"
		}
	}
	slim, ok := slimMarkdown(body)
	if !ok {
		return input, false, "markdown_unclosed_fence_preserved"
	}
	out := strings.Join(head, "\n") + "\n--\n" + strings.Join(slim, "\n")
	if len(out) >= len(input) || bytes.Equal([]byte(out), input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "markdown slimmed"
}

// slimMarkdown applies the removals to body lines. It reports false when a
// code fence is left open.
func slimMarkdown(body []string) ([]string, bool) {
	var out []string
	fence := ""
	lastHeading := ""
	for i := 0; i < len(body); i++ {
		l := body[i]
		if fence != "" {
			out = append(out, l)
			if m := reMdFence.FindStringSubmatch(l); m != nil && m[1][0] == fence[0] &&
				len(m[1]) >= len(fence) && strings.TrimSpace(l) == m[1] {
				fence = ""
			}
			continue
		}
		if m := reMdFence.FindStringSubmatch(l); m != nil {
			fence, lastHeading = m[1], ""
			out = append(out, l)
			continue
		}
		if reMdComment.MatchString(l) || isBadgeLine(l) {
			if prev := prevLine(out); len(out) == 0 || strings.TrimSpace(prev) == "" {
				i = skipBlank(body, i+1) - 1
			}
			continue
		}
		if strings.TrimSpace(l) == "" {
			j := skipBlank(body, i)
			if j-i > 1 && j < len(body) && !indentedCode(body[j]) && !indentedCode(prevLine(out)) {
				out = append(out, "")
				i = j - 1
				continue
			}
			out = append(out, l)
			continue
		}
		if reMdHeading.MatchString(l) {
			h := strings.TrimSpace(l)
			if h == lastHeading {
				i = skipBlank(body, i+1) - 1
				continue
			}
			lastHeading = h
		} else {
			lastHeading = ""
		}
		out = append(out, l)
	}
	return out, fence == ""
}

func prevLine(out []string) string {
	if len(out) == 0 {
		return ""
	}
	return out[len(out)-1]
}

func indentedCode(l string) bool {
	return strings.HasPrefix(l, "    ") || strings.HasPrefix(l, "\t")
}

// skipBlank returns the index of the first non-blank line at or after i.
func skipBlank(body []string, i int) int {
	for i < len(body) && strings.TrimSpace(body[i]) == "" {
		i++
	}
	return i
}

// isBadgeLine reports a line made only of badge images (shields.io and the
// like), alone or wrapped in links. A line with any other text is kept.
func isBadgeLine(l string) bool {
	t := strings.TrimSpace(l)
	if t == "" || !reMdBadgeItem.MatchString(t) {
		return false
	}
	return strings.TrimSpace(reMdBadgeItem.ReplaceAllString(t, "")) == ""
}
