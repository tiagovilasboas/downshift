// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package reportview

import (
	"fmt"
	"strconv"
	"strings"
)

var units = []struct {
	div    float64
	suffix string
}{{1e3, "k"}, {1e6, "M"}, {1e9, "B"}}

// Human formats a count for a narrow column: 950, 1.5k, 82k, 9.1M.
func Human(n int64) string {
	if n < 0 {
		return "-" + Human(-n)
	}
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}
	for i, u := range units {
		v := float64(n) / u.div
		s := strings.TrimSuffix(fmt.Sprintf("%.1f", v), ".0")
		if v >= 10 {
			s = fmt.Sprintf("%.0f", v)
		}
		if (v >= 1000 || s == "1000") && i+1 < len(units) {
			continue // move up a unit, also when 999,999 rounds to 1000
		}
		return s + u.suffix
	}
	return strconv.FormatInt(n, 10)
}

// pct formats a percentage without decimals.
func pct(p float64) string { return fmt.Sprintf("%.0f%%", p) }

const barCells = 10

// bar draws a barCells-wide bar for a 0-100 share.
func bar(p float64) string {
	n := int(p/100*barCells + 0.5)
	n = min(max(n, 0), barCells)
	return strings.Repeat("█", n) + strings.Repeat("░", barCells-n)
}
