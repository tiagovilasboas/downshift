// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// reLsRow reads one `ls -l` row: the type letter, the mode and an optional
// attribute mark, the link count, owner, group, size, date, then the name.
// Both date forms are read: BSD and GNU "Mon DD HH:MM" or "Mon DD YYYY", and
// GNU ISO "YYYY-MM-DD HH:MM[:SS[.frac] +ZZZZ]".
var reLsRow = lazyre.New(`^([-bcdlpsDCMnP?])[-rwxsStTlL]{9}[@+.]? +\d+ +\S+ +\S+ +(\d+(?:\.\d+)?[BKMGTPEZY]?) +` +
	`((?:[A-Za-z]{3} +\d{1,2} +(?:\d{1,2}:\d{2}|\d{4})|\d{4}-\d{2}-\d{2} +\d{2}:\d{2}(?::\d{2}(?:\.\d+)? [-+]\d{4})?)) +(.+)$`)

// reLsTotal is the first line of `ls -l` on a directory: its block count.
var reLsTotal = lazyre.New(`^total \d+$`)

type lsLongEntry struct {
	fileType string
	size     string
	date     string
	name     string
}

// compressLs compacts the output of `ls`. For plain listings, it keeps the
// names and drops . and .. entries. For long listings (`ls -l` / `ls -la`), it
// strips permissions, link count, owner, and group while right-aligning size
// and keeping timestamps, filenames, and symlink targets. Directories retain
// a trailing slash.
func compressLs(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, nil) {
		return input, false, "ls_danger_word_preserved"
	}
	lines := commandLines(input)
	if len(lines) == 0 {
		return input, false, "ls_empty_listing_preserved"
	}
	if reLsTotal.MatchString(lines[0]) || reLsRow.MatchString(lines[0]) {
		return compressLongLs(input, lines)
	}
	return compressPlainLs(input, lines)
}

func compressLongLs(input []byte, lines []string) ([]byte, bool, string) {
	start := 0
	if reLsTotal.MatchString(lines[0]) {
		start = 1
	}
	var entries []lsLongEntry
	maxSizeLen := 0
	for i := start; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			return input, false, "ls_unrecognized_structure_preserved"
		}
		m := reLsRow.FindStringSubmatch(line)
		if m == nil {
			return input, false, "ls_unrecognized_structure_preserved"
		}
		fileType := m[1]
		size := m[2]
		date := m[3]
		name := m[4]

		if name == "." || name == ".." {
			continue
		}
		if fileType == "d" && !strings.HasSuffix(name, "/") {
			name += "/"
		}
		if len(size) > maxSizeLen {
			maxSizeLen = len(size)
		}
		entries = append(entries, lsLongEntry{
			fileType: fileType,
			size:     size,
			date:     date,
			name:     name,
		})
	}
	if len(entries) == 0 {
		return input, false, "ls_empty_listing_preserved"
	}
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%*s %s %s\n", maxSizeLen, e.size, e.date, e.name)
	}
	out := []byte(b.String())
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "ls long form compacted"
}

func compressPlainLs(input []byte, lines []string) ([]byte, bool, string) {
	var names []string
	for _, line := range lines {
		if line == "" {
			return input, false, "ls_unrecognized_structure_preserved"
		}
		if reLsRow.MatchString(line) {
			return input, false, "ls_unrecognized_structure_preserved"
		}
		if line == "." || line == ".." {
			continue
		}
		names = append(names, line)
	}
	if len(names) == 0 {
		return input, false, "ls_empty_listing_preserved"
	}
	out := []byte(strings.Join(names, "\n") + "\n")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, "ls names only"
}
