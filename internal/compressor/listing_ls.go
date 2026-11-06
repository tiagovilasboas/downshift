// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"regexp"
	"strings"
)

// reLsRow reads one `ls -l` row: the type letter, the mode and an optional
// attribute mark, the link count, owner, group, size, date, then the name. It
// only recognises a long row, so the compressor can leave it raw.
// Both date forms are read: BSD and GNU "Mon DD HH:MM" or "Mon DD YYYY", and
// GNU ISO "YYYY-MM-DD HH:MM[:SS[.frac] +ZZZZ]".
var reLsRow = regexp.MustCompile(`^([-bcdlpsDCMnP?])[-rwxsStTlL]{9}[@+.]? +\d+ +\S+ +\S+ +\d+ +` +
	`(?:[A-Za-z]{3} +\d{1,2} +(?:\d{1,2}:\d{2}|\d{4})|\d{4}-\d{2}-\d{2} +\d{2}:\d{2}(?::\d{2}(?:\.\d+)? [-+]\d{4})?) (.+)$`)

// reLsTotal is the first line of `ls -l` on a directory: its block count.
var reLsTotal = regexp.MustCompile(`^total \d+$`)

// compressLs keeps the names of a plain listing and drops nothing else. A long
// listing (`ls -l`) is kept raw: its type, mode, size and mtime columns are
// the facts the agent asked for, and dropping only owner, group and link count
// saves too little to justify a custom reader. Plain names are lossy only in
// the `.` and `..` entries, so the danger gate still runs first: any danger
// word keeps the listing raw. A directory keeps its trailing slash (`ls -p`).
// A row that does not read as a plain name, an empty line, a listing with no
// entry, or a listing where nothing is gained keeps the output raw.
func compressLs(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, nil) {
		return input, false, "ls_danger_word_preserved"
	}
	lines := commandLines(input)
	if reLsTotal.MatchString(lines[0]) {
		return input, false, "ls_long_form_kept_raw"
	}
	var names []string
	for _, line := range lines {
		if line == "" {
			return input, false, "ls_unrecognized_structure_preserved"
		}
		if reLsRow.MatchString(line) {
			return input, false, "ls_long_form_kept_raw"
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
