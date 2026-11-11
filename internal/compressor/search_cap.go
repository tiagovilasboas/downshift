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
)

// CapSearchRows keeps the first searchCapPerFile rows of each file and
// searchCapTotal rows in all, grouped by file in the order the search first
// printed it. A row's file is its text before the first colon ("path:line:
// text"); rows with no colon count as one group. Once a row is dropped, every
// kept row longer than searchCapCells is cut there and says how much it lost.
// Each file that lost rows ends with a count, and one line counts the files
// left out whole. It reports false, and the caller keeps text, when no row is
// dropped or the result is not shorter. The caller stores the raw text first:
// every dropped byte comes back with `downshift retrieve`.
func CapSearchRows(text []byte) ([]byte, bool) {
	lines := strings.Split(strings.TrimSuffix(string(text), "\n"), "\n")
	var order []string
	rows := map[string][]string{}
	for _, l := range lines {
		f := matchFile(l)
		if _, ok := rows[f]; !ok {
			order = append(order, f)
		}
		rows[f] = append(rows[f], l)
	}
	var b strings.Builder
	shown, dropped, leftFiles, leftRows := 0, false, 0, 0
	for _, f := range order {
		if shown >= searchCapTotal {
			leftFiles++
			leftRows += len(rows[f])
			continue
		}
		n := min(min(len(rows[f]), searchCapPerFile), searchCapTotal-shown)
		dropped = dropped || n < len(rows[f])
		shown += n
	}
	if !dropped && leftFiles == 0 {
		return text, false
	}
	shown = 0
	for _, f := range order[:len(order)-leftFiles] {
		rs := rows[f]
		n := min(min(len(rs), searchCapPerFile), searchCapTotal-shown)
		for _, l := range rs[:n] {
			b.WriteString(capRow(l) + "\n")
		}
		shown += n
		if more := len(rs) - n; more > 0 {
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

// groupName names a group in its count line.
func groupName(f string) string {
	if f == "" {
		return "lines without a file"
	}
	return f
}

// capRow cuts a row longer than searchCapCells, never inside a character.
func capRow(l string) string {
	if len(l) <= searchCapCells {
		return l
	}
	n := searchCapCells
	for n > 0 && !utf8.RuneStart(l[n]) {
		n--
	}
	return l[:n] + fmt.Sprintf(" … (%d more bytes)", len(l)-n)
}
