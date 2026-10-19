// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxProcessableBytes prevents unbounded memory usage.
	MaxProcessableBytes = 2 * 1024 * 1024 // 2 MB
)

var (
	reGoTestPass    = regexp.MustCompile(`(?m)^ok\s+([^\s]+)\s+([^\s]+)(?:\s+coverage:\s+(.+))?$`)
	reGoTestFail    = regexp.MustCompile(`(?m)^(?:--- FAIL:|FAIL|\s*panic:)`)
	reGitLogCommit  = regexp.MustCompile(`(?m)^commit\s+[0-9a-f]{40}`)
	reGitStatusHead = regexp.MustCompile(`(?m)^(?:On branch |HEAD detached at )`)
	reRipgrepMatch  = regexp.MustCompile(`(?m)^(?:\d+:)?[^:\n]+:\d+:`)
)

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

	// Output that does not end in a newline may have been cut off mid-line,
	// and CRLF output would be rewritten with stray carriage returns. In both
	// cases the raw bytes are the only faithful answer.
	if input[len(input)-1] != '\n' {
		res.Reason = "no_final_newline_preserved"
		return res
	}
	if bytes.IndexByte(input, '\r') != -1 {
		res.Reason = "carriage_return_preserved"
		return res
	}

	// Detect format
	format, isError := detectFormat(input)
	res.Format = format

	// Safety Rule: Fail-open on error.
	// Never summarize or alter errors, stack traces, or failing test suites.
	if isError {
		res.Reason = "error_detected_fail_open_raw_preserved"
		return res
	}

	// Attempt format-specific compression
	compacted, applied, reason := applyFormatCompression(input, format)
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

func detectFormat(input []byte) (Format, bool) {
	// 1. Go test
	if reGoTestFail.Match(input) {
		return FormatGoTest, true // Error detected! Preserve raw!
	}
	if bytes.Contains(input, []byte("PASS\n")) || reGoTestPass.Match(input) {
		return FormatGoTest, false
	}

	// 2. Git status
	if reGitStatusHead.Match(input) {
		return FormatGitStatus, false
	}

	// 3. Git log
	if reGitLogCommit.Match(input) {
		return FormatGitLog, false
	}

	// 4. Repetitive logs, only after go test, git status and git log had their
	// say: a failing test run or a git log with repeated lines must stay raw.
	// Timestamps can look like search hits, so this precedes the search check.
	if isRepetitiveLog(input) {
		return FormatLogs, false
	}

	// 5. Search results (rg / grep)
	if reRipgrepMatch.Match(input) {
		return FormatSearch, false
	}

	// 6. File listings / tree
	if bytes.Contains(input, []byte("total ")) && bytes.Contains(input, []byte("drwx")) {
		return FormatFileList, false
	}

	return FormatUnknown, false
}

func applyFormatCompression(input []byte, f Format) ([]byte, bool, string) {
	switch f {
	case FormatGoTest:
		return compressGoTest(input)
	case FormatGitStatus:
		return compressGitStatus(input)
	case FormatGitLog:
		return compressGitLog(input)
	case FormatSearch:
		return compressSearchResults(input)
	case FormatFileList:
		return compressFileList(input)
	case FormatLogs:
		return deduplicateLogs(input)
	default:
		return input, false, "unrecognized_format"
	}
}

// compressGoTest summarises a suite only when every non-empty line is an
// "ok" package line. Any other line (warning, skip, fail text the detector
// missed) keeps the original output. The summary keeps the pass count, every
// package name (the module prefix shared by all of them is stated once) and
// the coverage figure when present; it drops only timings and "(cached)".
// If that is not strictly shorter than the input, the input is returned.
func compressGoTest(input []byte) ([]byte, bool, string) {
	type pkg struct{ name, cover string }
	var pkgs []pkg
	for _, line := range strings.Split(string(input), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := reGoTestPass.FindStringSubmatch(line)
		if m == nil {
			return input, false, "go_test_preserved_non_pass_lines"
		}
		pkgs = append(pkgs, pkg{name: m[1], cover: strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(m[3]), "of statements"))})
	}
	if len(pkgs) < 2 {
		return input, false, "go_test_too_few_packages_to_compress"
	}
	names := make([]string, len(pkgs))
	for i, p := range pkgs {
		names[i] = p.name
	}
	prefix := commonPathPrefix(names)

	var b strings.Builder
	fmt.Fprintf(&b, "Go test: PASS (%d packages ok)", len(pkgs))
	if prefix != "" {
		fmt.Fprintf(&b, " under %s", prefix)
	}
	b.WriteString("\n")
	for _, p := range pkgs {
		b.WriteString("ok " + strings.TrimPrefix(p.name, prefix))
		if p.cover != "" {
			b.WriteString(" " + p.cover)
		}
		b.WriteString("\n")
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed %d passing packages into summary", len(pkgs))
}

// commonPathPrefix returns the longest prefix shared by all names that ends
// at a '/' boundary (including the slash), or "" when there is none.
func commonPathPrefix(names []string) string {
	prefix := names[0]
	for _, n := range names[1:] {
		for !strings.HasPrefix(n, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	i := strings.LastIndexByte(prefix, '/')
	if i < 0 {
		return ""
	}
	return prefix[:i+1]
}

// compressGitStatus squelches a clean working tree, and only when every
// non-empty line is the branch line, an "up to date" tracking line or the
// clean marker. Ahead, behind, diverged, detached or any other line keeps the
// original output: an agent deciding whether to push needs that line.
func compressGitStatus(input []byte) ([]byte, bool, string) {
	s := string(input)
	if !strings.Contains(s, "nothing to commit, working tree clean") {
		return input, false, "git_status_has_changes_preserved"
	}
	branch := ""
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "On branch ") && branch == "":
			branch = line
		case strings.HasPrefix(line, "Your branch is up to date"):
		case line == "nothing to commit, working tree clean":
		default:
			return input, false, "git_status_extra_lines_preserved"
		}
	}
	if branch == "" {
		return input, false, "git_status_no_branch_preserved"
	}
	return []byte(fmt.Sprintf("%s (working tree clean)\n", branch)), true, "squelched clean working tree"
}

// compressGitLog keeps the raw log. An oneline summary drops bodies.
func compressGitLog(input []byte) ([]byte, bool, string) {
	return input, false, "git_log_preserved"
}

// compressSearchResults does not drop matches. A prefix would hide hits.
func compressSearchResults(input []byte) ([]byte, bool, string) {
	return input, false, "search_results_preserved"
}

// compressFileList does not drop names. A prefix would hide entries.
func compressFileList(input []byte) ([]byte, bool, string) {
	return input, false, "file_list_preserved"
}

// isRepetitiveLog checks if successive lines have high duplicate frequency.
func isRepetitiveLog(input []byte) bool {
	lines := strings.Split(string(input), "\n")
	if len(lines) < 10 {
		return false
	}
	repeats := 0
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" && lines[i] == lines[i-1] {
			repeats++
		}
	}
	return repeats >= 4
}

// deduplicateLogs collapses repeated consecutive log lines into "[repeated N times]".
func deduplicateLogs(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")
	if len(lines) == 0 {
		return input, false, "empty"
	}
	var out []string
	curLine := lines[0]
	count := 1

	for i := 1; i < len(lines); i++ {
		if lines[i] == curLine && curLine != "" {
			count++
		} else {
			if count > 1 {
				out = append(out, curLine, fmt.Sprintf("  ↳ [repeated %d times]", count-1))
			} else {
				out = append(out, curLine)
			}
			curLine = lines[i]
			count = 1
		}
	}
	if count > 1 {
		out = append(out, curLine, fmt.Sprintf("  ↳ [repeated %d times]", count-1))
	} else if curLine != "" {
		out = append(out, curLine)
	}

	res := strings.Join(out, "\n") + "\n"
	if len(res) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(res), true, "deduplicated consecutive log lines"
}
