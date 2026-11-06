// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package configfile

import (
	"fmt"
	"strings"
)

// UnifiedDiff returns a unified-style diff (3 lines of context) between two
// texts, or "" when they are equal.
func UnifiedDiff(oldName, newName, oldText, newText string) string {
	if oldText == newText {
		return ""
	}
	a, b := splitLines(oldText), splitLines(newText)
	ops := diffOps(a, b)

	const ctx = 3
	var out strings.Builder
	fmt.Fprintf(&out, "--- %s\n+++ %s\n", oldName, newName)
	n := len(ops)
	for i := 0; i < n; {
		if ops[i].kind == ' ' {
			i++
			continue
		}
		start := i - ctx
		if start < 0 {
			start = 0
		}
		// Extend the hunk while the next change is within 2*ctx lines.
		last := i
		for k := last + 1; k < n; k++ {
			if ops[k].kind == ' ' {
				continue
			}
			if k-last-1 > 2*ctx {
				break
			}
			last = k
		}
		stop := last + 1 + ctx
		if stop > n {
			stop = n
		}
		aStart, bStart := 1, 1
		aLen, bLen := 0, 0
		if start < n {
			aStart, bStart = ops[start].aLine, ops[start].bLine
		}
		var body strings.Builder
		for _, op := range ops[start:stop] {
			switch op.kind {
			case ' ':
				aLen++
				bLen++
			case '-':
				aLen++
			case '+':
				bLen++
			}
			body.WriteByte(op.kind)
			body.WriteString(op.text)
			body.WriteByte('\n')
		}
		if aLen == 0 {
			aStart--
		}
		if bLen == 0 {
			bStart--
		}
		fmt.Fprintf(&out, "@@ -%d,%d +%d,%d @@\n%s", aStart, aLen, bStart, bLen, body.String())
		i = stop
	}
	return out.String()
}

type diffOp struct {
	kind         byte // ' ', '-', '+'
	text         string
	aLine, bLine int // 1-based line numbers where this op sits in each side
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

// diffOps builds an edit script with a longest-common-subsequence table.
// Settings files are small, so the O(n*m) table is fine.
func diffOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}
	var ops []diffOp
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			ops = append(ops, diffOp{' ', a[i], i + 1, j + 1})
			i++
			j++
		case j >= m || (i < n && lcs[i+1][j] >= lcs[i][j+1]):
			ops = append(ops, diffOp{'-', a[i], i + 1, j + 1})
			i++
		default:
			ops = append(ops, diffOp{'+', b[j], i + 1, j + 1})
			j++
		}
	}
	return ops
}
