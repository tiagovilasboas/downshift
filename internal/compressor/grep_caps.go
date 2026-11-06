// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

// Caps for grouped search output. A search that matches everywhere (a common
// word over a whole tree) would otherwise print thousands of rows; the cap
// keeps the first matches of each file and says what it cut, so the reader
// can narrow the search. 20 rows per file and 200 in all is about 16 KB, a
// few thousand tokens, and well above what a targeted search prints (see
// docs/reference/supported-tools.md).
const (
	grepPerFileCap = 20
	grepTotalCap   = 200
)

// matchGroups holds search rows grouped under their file, in the order the
// search found the files.
type matchGroups struct {
	files []string
	rows  map[string][]string
}

func newMatchGroups() *matchGroups { return &matchGroups{rows: map[string][]string{}} }

// add appends one row under file.
func (g *matchGroups) add(file, row string) {
	if _, ok := g.rows[file]; !ok {
		g.files = append(g.files, file)
	}
	g.rows[file] = append(g.rows[file], row)
}

// render prints each file followed by its rows, one space deep. Past
// grepPerFileCap rows in one file, or grepTotalCap rows in all, the rest are
// cut and counted: a line closes the file ("... N more in this file") and a
// final line counts the files left out whole.
func (g *matchGroups) render() string {
	var b strings.Builder
	left := grepTotalCap
	wholeRows, wholeFiles := 0, 0
	for _, f := range g.files {
		rows := g.rows[f]
		show := min(min(len(rows), grepPerFileCap), left)
		if show == 0 {
			wholeRows += len(rows)
			wholeFiles++
			continue
		}
		b.WriteString(f + "\n")
		for _, r := range rows[:show] {
			b.WriteString(" " + r + "\n")
		}
		if more := len(rows) - show; more > 0 {
			fmt.Fprintf(&b, " ... %d more in this file\n", more)
		}
		left -= show
	}
	if wholeFiles > 0 {
		fmt.Fprintf(&b, "... %d more matches in %d more files (narrow the search, or add -l / -c)\n", wholeRows, wholeFiles)
	}
	return b.String()
}
