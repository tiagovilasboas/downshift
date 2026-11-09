// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package desktopview

import (
	"fmt"
	"strings"
)

// op is one line of a line diff: ' ' kept, '-' only in the base, '+' only in
// the current view. cur is the 1-based line of the current view a kept or
// added line is on (0 for a removed line).
type op struct {
	kind byte
	text string
	cur  int
}

// diffLines is a longest-common-subsequence line diff of a and b. The common
// head and tail are matched first; it reports false when the rest is larger
// than maxDiffCells.
func diffLines(a, b []string) ([]op, bool) {
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	ma, mb := a[head:len(a)-tail], b[head:len(b)-tail]
	if (len(ma)+1)*(len(mb)+1) > maxDiffCells {
		return nil, false
	}
	ops := make([]op, 0, len(a)+len(b))
	for i := 0; i < head; i++ {
		ops = append(ops, op{' ', b[i], i + 1})
	}
	ops = append(ops, lcsOps(ma, mb, head)...)
	for i := len(b) - tail; i < len(b); i++ {
		ops = append(ops, op{' ', b[i], i + 1})
	}
	return ops, true
}

// lcsOps diffs the middle region; offset is the number of lines before it.
func lcsOps(a, b []string, offset int) []op {
	n, m := len(a), len(b)
	l := make([][]int32, n+1)
	for i := range l {
		l[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				l[i][j] = l[i+1][j+1] + 1
			} else {
				l[i][j] = max(l[i+1][j], l[i][j+1])
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			ops = append(ops, op{' ', b[j], offset + j + 1})
			i, j = i+1, j+1
		case i < n && (j == m || l[i+1][j] >= l[i][j+1]):
			ops = append(ops, op{'-', a[i], 0})
			i++
		default:
			ops = append(ops, op{'+', b[j], offset + j + 1})
			j++
		}
	}
	return ops
}

// writeHunks writes each run of changed lines with the kept line before it
// as context, under "@@ line N" (N is the line of the current view the hunk
// starts at), and returns the number of changed lines.
func writeHunks(out *strings.Builder, ops []op, cur []string) int {
	changed := 0
	for k := 0; k < len(ops); k++ {
		if ops[k].kind == ' ' {
			continue
		}
		start := k
		if k > 0 {
			start = k - 1 // the kept line before the change
		}
		fmt.Fprintf(out, "@@ line %d\n", hunkLine(ops, start))
		for _, o := range ops[start:k] {
			out.WriteString(" " + o.text + "\n")
		}
		for ; k < len(ops) && ops[k].kind != ' '; k++ {
			out.WriteString(string(ops[k].kind) + ops[k].text + "\n")
			changed++
		}
	}
	return changed
}

// hunkLine is the current-view line a hunk starting at ops[k] is on: the
// first kept or added line from k, or the line after the last one before it.
func hunkLine(ops []op, k int) int {
	for i := k; i < len(ops); i++ {
		if ops[i].cur > 0 {
			return ops[i].cur
		}
	}
	last := 0
	for _, o := range ops {
		if o.cur > last {
			last = o.cur
		}
	}
	return last + 1
}
