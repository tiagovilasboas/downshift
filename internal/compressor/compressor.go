// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
func Compress(input []byte, mode Mode) Result {
	origLen := len(input)
	res := Result{
		Output:        input,
		Applied:       false,
		Format:        FormatUnknown,
		OriginalBytes: origLen,
		ReducedBytes:  origLen,
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
	// Repetitive logs
	if isRepetitiveLog(input) {
		return FormatLogs, false
	}

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

	// 4. Search results (rg / grep)
	if reRipgrepMatch.Match(input) {
		return FormatSearch, false
	}

	// 5. File listings / tree
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

// compressGoTest aggregates passed packages into a single summary line.
func compressGoTest(input []byte) ([]byte, bool, string) {
	matches := reGoTestPass.FindAllSubmatch(input, -1)
	if len(matches) < 2 {
		return input, false, "go_test_too_few_packages_to_compress"
	}
	summary := fmt.Sprintf("Go test: PASS (%d packages ok)\n", len(matches))
	return []byte(summary), true, fmt.Sprintf("compressed %d passing packages into summary", len(matches))
}

// compressGitStatus squelches "nothing to commit" or long untracked blocks.
func compressGitStatus(input []byte) ([]byte, bool, string) {
	s := string(input)
	if strings.Contains(s, "nothing to commit, working tree clean") {
		branch := "branch"
		lines := strings.Split(s, "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "On branch ") {
			branch = lines[0]
		}
		return []byte(fmt.Sprintf("%s (working tree clean)\n", branch)), true, "squelched clean working tree"
	}
	return input, false, "git_status_has_changes_preserved"
}

// compressGitLog summarizes multi-commit logs into oneline format.
func compressGitLog(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")
	var oneline []string
	var curHash, curSubject string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "commit ") && len(trimmed) >= 15 {
			if curHash != "" {
				oneline = append(oneline, fmt.Sprintf("%.7s %s", curHash, curSubject))
			}
			curHash = strings.TrimPrefix(trimmed, "commit ")
			curSubject = ""
		} else if curHash != "" && curSubject == "" && trimmed != "" && !strings.HasPrefix(trimmed, "Author:") && !strings.HasPrefix(trimmed, "Date:") {
			curSubject = trimmed
		}
	}
	if curHash != "" {
		oneline = append(oneline, fmt.Sprintf("%.7s %s", curHash, curSubject))
	}

	if len(oneline) < 3 {
		return input, false, "git_log_too_short"
	}
	out := strings.Join(oneline, "\n") + "\n"
	return []byte(out), true, fmt.Sprintf("compacted %d commits to oneline", len(oneline))
}

// compressSearchResults limits high-volume search matches to first N lines + total count.
func compressSearchResults(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")
	if len(lines) <= 25 {
		return input, false, "search_results_under_threshold"
	}
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf("... [%d more matches truncated]\n", len(lines)-20))
	return []byte(b.String()), true, fmt.Sprintf("truncated %d search lines to 20", len(lines))
}

// compressFileList aggregates extensive ls listings.
func compressFileList(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")
	if len(lines) <= 30 {
		return input, false, "file_list_under_threshold"
	}
	var b strings.Builder
	for i := 0; i < 15; i++ {
		b.WriteString(lines[i])
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf("... [%d entries omitted]\n", len(lines)-15))
	return []byte(b.String()), true, fmt.Sprintf("truncated %d file entries to 15", len(lines))
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
