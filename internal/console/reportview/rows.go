// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package reportview

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

// seg is a piece of one line with its style.
type seg struct {
	text  string
	style lipgloss.Style
}

// compose joins segments into one line no wider than width, clipping the plain
// text before it is styled.
func (m Model) compose(width int, segs ...seg) string {
	var b strings.Builder
	left := width
	for _, s := range segs {
		text := s.text
		if width > 0 {
			if left <= 0 {
				break
			}
			text = frame.Clip(text, left)
			left -= lipgloss.Width(text)
		}
		if m.th.NoColor {
			b.WriteString(text) // bold alone would still emit an escape
			continue
		}
		b.WriteString(s.style.Render(text))
	}
	return b.String()
}

// barsFit reports whether the width leaves room for the bars.
func barsFit(width int) bool { return theme.BreakpointFor(width) != theme.Narrow }

// rule is a harness title followed by a line to the edge.
func (m Model) rule(harness string) string {
	n := max(0, min(m.width, 72)-len(harness)-1)
	return m.compose(m.width, seg{harness, m.th.Accent}, seg{" " + strings.Repeat("─", n), m.th.Muted})
}

// section is a small heading inside a harness block.
func (m Model) section(title string) string {
	return m.compose(m.width, seg{"  " + title, m.th.Info})
}

// textRow is a label, a value and an optional muted context.
func (m Model) textRow(label, value, ctx string) string {
	segs := []seg{{fmt.Sprintf("  %-*s", labelCells, label), m.th.Bold}, {value, m.th.Bold}}
	if ctx != "" {
		segs = append(segs, seg{"  " + ctx, m.th.Muted})
	}
	return m.compose(m.width, segs...)
}

// metric is a label with a share bar, its value and a muted context. An
// unobserved figure reads "unobserved", never 0% and never a bar.
func (m Model) metric(bars bool, label string, observed bool, share float64, value, ctx string, st lipgloss.Style) string {
	segs := []seg{{fmt.Sprintf("  %-*s", labelCells, label), m.th.Bold}}
	if !observed {
		return m.compose(m.width, append(segs, seg{"unobserved", m.th.Muted})...)
	}
	if bars {
		segs = append(segs, seg{bar(share) + "  ", st})
	}
	segs = append(segs, seg{fmt.Sprintf("%-4s", value), m.th.Bold})
	if ctx != "" {
		segs = append(segs, seg{"  " + ctx, m.th.Muted})
	}
	return m.compose(m.width, segs...)
}
