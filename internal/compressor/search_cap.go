// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Caps of CapSearchRows. Measured over 14 days of real search output led by
// grep, rg or git grep (2026-11-11): at 25 rows per file no later tool call
// needed a dropped row, while 10 per file missed 4% and 5 per file 9%.
const (
	searchCapPerFile = 25 // rows kept per file
	searchCapTotal   = 200
	searchCapCells   = 80 // bytes a kept row keeps once rows are dropped
	searchCutMark    = " …"
	// searchCapMinParsed is the share, in tenths, of non-separator lines that
	// must parse as a file row before anything is cut. File lists (grep -l)
	// and counted lists (uniq -c) parse as nothing; 9 tenths tolerates the
	// odd "Binary file x matches" line in real search output.
	searchCapMinParsed = 9
	// searchCapMaxLoose is the longest run of lines that are not rows: a
	// longer run is text after the search (`grep x && cat f`), never cut.
	searchCapMaxLoose = 3
	searchSeparator   = "--"
)

// searchGroup holds the lines of one file in original order; rows counts the
// lines that are not "--" separators.
type searchGroup struct {
	lines []string
	rows  int
}

// CapSearchRows keeps the first searchCapPerFile rows of each file and
// searchCapTotal rows in all, grouped by file in the order the search first
// printed it. A row is "path:N:text", or a context line "path-N-text" of a
// path that has a match; a Windows drive prefix belongs to the path. "--"
// separators stay with the file before them and are kept only between kept
// rows. Lines that do not parse follow the row before them. Once a row is
// dropped, every kept row longer than searchCapCells is cut there. Each file
// that lost rows ends with a count, and one line counts the files left out
// whole. It reports false, and the caller keeps text, when fewer than
// searchCapMinParsed tenths of the lines are file rows, when no row is
// dropped, or when the result is not shorter. The caller stores the raw text
// first: every dropped byte comes back with `downshift retrieve`.
func CapSearchRows(text []byte) ([]byte, bool) {
	groups, order, ok := groupSearchRows(strings.Split(strings.TrimSuffix(string(text), "\n"), "\n"))
	if !ok {
		return text, false
	}
	var b strings.Builder
	shown, dropped, leftFiles, leftRows := 0, false, 0, 0
	for _, f := range order {
		if shown >= searchCapTotal {
			leftFiles++
			leftRows += groups[f].rows
			continue
		}
		n := min(min(groups[f].rows, searchCapPerFile), searchCapTotal-shown)
		dropped = dropped || n < groups[f].rows
		shown += n
	}
	if !dropped && leftFiles == 0 {
		return text, false
	}
	shown = 0
	for _, f := range order[:len(order)-leftFiles] {
		g := groups[f]
		n := min(min(g.rows, searchCapPerFile), searchCapTotal-shown)
		writeGroup(&b, g, n)
		shown += n
		if more := g.rows - n; more > 0 {
			fmt.Fprintf(&b, "  … %d more in %s\n", more, groupName(f))
		}
	}
	if leftFiles > 0 {
		fmt.Fprintf(&b, "… %d more matches in %d more files\n", leftRows, leftFiles)
	}
	if b.Len() >= len(text) {
		return text, false
	}
	return []byte(b.String()), true
}

// groupSearchRows groups lines by file. It reports false when too few lines
// are file rows for this to be search output.
func groupSearchRows(lines []string) (map[string]*searchGroup, []string, bool) {
	known := map[string]bool{}
	for _, l := range lines {
		if k, ok := matchRowKey(l); ok {
			known[k] = true
		}
	}
	groups := map[string]*searchGroup{}
	var order []string
	cur, parsed, total, loose := "", 0, 0, 0
	for _, l := range lines {
		k, ok, sep := cur, false, isSeparator(l)
		if !sep {
			total++
			if k, ok = contextOrMatchKey(l, known); ok {
				parsed++
				loose = 0
			} else if k, loose = cur, loose+1; loose > searchCapMaxLoose {
				return nil, nil, false
			}
		}
		g := groups[k]
		if g == nil {
			if sep {
				continue // nothing before it to separate
			}
			g = &searchGroup{}
			groups[k] = g
			order = append(order, k)
		}
		g.lines = append(g.lines, l)
		if !sep {
			g.rows++
		}
		cur = k
	}
	return groups, order, total > 0 && parsed*10 >= total*searchCapMinParsed
}

// writeGroup writes the first n rows of g, a separator only between rows.
func writeGroup(b *strings.Builder, g *searchGroup, n int) {
	written, pending := 0, false
	for _, l := range g.lines {
		if written == n {
			return
		}
		if isSeparator(l) {
			pending = written > 0
			continue
		}
		if pending {
			b.WriteString(searchSeparator + "\n")
			pending = false
		}
		b.WriteString(capRow(l) + "\n")
		written++
	}
}

// isSeparator reports whether l is grep's "--" group separator, CRLF or not.
func isSeparator(l string) bool { return strings.TrimSuffix(l, "\r") == searchSeparator }

// splitDrive splits a Windows drive prefix ("C:\" or "C:/") off a row.
func splitDrive(l string) (drive, rest string) {
	if len(l) >= 3 && l[1] == ':' && (l[2] == '\\' || l[2] == '/') &&
		(l[0]|0x20 >= 'a' && l[0]|0x20 <= 'z') {
		return l[:2], l[2:]
	}
	return "", l
}

// digitsEnd returns the index after the digits that start s[i:], or i if none.
func digitsEnd(s string, i int) int {
	j := i
	for j < len(s) && s[j] >= '0' && s[j] <= '9' {
		j++
	}
	return j
}

// matchRowKey returns the file of a "path:N:text" row.
func matchRowKey(l string) (string, bool) {
	drive, rest := splitDrive(l)
	i := strings.IndexByte(rest, ':')
	if i <= 0 {
		return "", false
	}
	j := digitsEnd(rest, i+1)
	if j == i+1 || j >= len(rest) || rest[j] != ':' || strings.ContainsAny(rest[:i], " \t") {
		return "", false
	}
	return drive + rest[:i], true
}

// contextOrMatchKey returns the file of a match row or of a "path-N-text"
// context line whose path is in known.
func contextOrMatchKey(l string, known map[string]bool) (string, bool) {
	drive, rest := splitDrive(l)
	for i := 1; i < len(rest); i++ {
		if rest[i] != '-' {
			continue
		}
		if j := digitsEnd(rest, i+1); j > i+1 && j < len(rest) && rest[j] == '-' && known[drive+rest[:i]] {
			return drive + rest[:i], true
		}
	}
	return matchRowKey(l)
}

// groupName names a group in its count line.
func groupName(f string) string {
	if f == "" {
		return "lines without a file"
	}
	return f
}

// capRow cuts a row longer than searchCapCells, never inside a character.
// A row only a few bytes over is kept: the mark would not make it shorter.
func capRow(l string) string {
	if len(l) <= searchCapCells+len(searchCutMark) {
		return l
	}
	n := searchCapCells
	for n > 0 && !utf8.RuneStart(l[n]) {
		n--
	}
	return l[:n] + searchCutMark
}
