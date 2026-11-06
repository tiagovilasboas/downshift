// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Git log compressor.
//
// The default `git log` shape is one block per commit: a "commit <hash>" line
// (optionally followed by decorations), an optional "Merge:" line, "Author:"
// and "Date:" lines, a blank line and the indented message. What an agent reads
// a log for is the hash, the refs and the subject, so each commit becomes one
// line: 7-char hash, decorations if present, subject verbatim. Author, date,
// merge parents and message bodies are dropped (owner decision, 2026-11-04,
// replacing the earlier "git log is never summarised" contract). Dropping the
// body is deliberate even when it mentions fail/panic: the subject is the
// commit's own summary and stays verbatim.
//
// Any line that does not fit the shape above (a stat block, a patch, gpg
// signature lines, a custom --format) makes the whole input pass through raw.

// reGitLogCommitLine matches "commit <40|64 hex>" plus optional decorations
// such as " (HEAD -> main, tag: v1)".
var reGitLogCommitLine = regexp.MustCompile(`^commit ([0-9a-f]{40}|[0-9a-f]{64})(?: (\(.*\)))?$`)

const gitLogShortHash = 7

// gitLogEntry is one parsed commit: what survives into the output.
type gitLogEntry struct {
	hash    string
	decor   string
	subject string
	author  bool
}

func (e gitLogEntry) line() string {
	l := e.hash
	if e.decor != "" {
		l += " " + e.decor
	}
	if e.subject != "" {
		l += " " + e.subject
	}
	return l
}

// isGitLog reports whether input is, start to end, a default-format git log. It
// runs before the test-runner heuristics: a commit body that quotes "FAIL" or
// "panic:" would otherwise be taken for a failing go test run. The strict parse
// keeps this safe; any other shape falls through to the usual detection.
func isGitLog(input []byte) bool {
	if !bytes.HasPrefix(input, []byte("commit ")) {
		return false
	}
	_, ok := parseGitLog(string(input))
	return ok
}

// compressGitLog rewrites a default-format git log to one line per commit.
func compressGitLog(input []byte) ([]byte, bool, string) {
	entries, ok := parseGitLog(string(input))
	if !ok {
		return input, false, "git_log_unrecognized_structure_preserved"
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(e.line() + "\n")
	}
	if b.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(b.String()), true, fmt.Sprintf("compressed git log (%d commits)", len(entries))
}

// parseGitLog parses the log strictly; ok is false on any unexpected line.
func parseGitLog(s string) ([]gitLogEntry, bool) {
	var entries []gitLogEntry
	inMessage := false
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if m := reGitLogCommitLine.FindStringSubmatch(line); m != nil {
			entries = append(entries, gitLogEntry{hash: m[1][:gitLogShortHash], decor: m[2]})
			inMessage = false
			continue
		}
		if len(entries) == 0 {
			return nil, false
		}
		cur := &entries[len(entries)-1]
		switch {
		case !inMessage && strings.HasPrefix(line, "Author: "):
			cur.author = true
		case !inMessage && (strings.HasPrefix(line, "Merge: ") || strings.HasPrefix(line, "Date:   ")):
		case line == "":
			inMessage = true
		case inMessage && strings.HasPrefix(line, "    "):
			if cur.subject == "" {
				cur.subject = line[4:]
			}
		default:
			return nil, false
		}
	}
	for _, e := range entries {
		if !e.author {
			return nil, false
		}
	}
	return entries, len(entries) > 0
}
