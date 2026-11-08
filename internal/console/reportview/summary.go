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

const labelCells = 24

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

func (m Model) summaryHeader() string {
	h := m.data.Header
	title := h.Label
	if title == "" {
		title = "Downshift"
	}
	return fmt.Sprintf("%s · last %d days · %d commands · %d compacted", title, h.Days, h.Commands, h.Compacted)
}

// summaryLines draws each harness with its four metrics. Nothing is summed.
func (m Model) summaryLines() []string {
	if len(m.data.Harnesses) == 0 {
		return []string{m.compose(m.width, seg{"No harness activity in this window.", m.th.Muted})}
	}
	var out []string
	most := int64(0)
	for _, r := range m.data.Harnesses {
		most = max(most, r.TokensSaved)
	}
	bars := theme.BreakpointFor(m.width) != theme.Narrow
	for i, r := range m.data.Harnesses {
		if i > 0 && !(compactOnly(r) && compactOnly(m.data.Harnesses[i-1])) {
			out = append(out, "")
		}
		saved := 0.0
		if most > 0 {
			saved = float64(r.TokensSaved) / float64(most) * 100
		}
		if compactOnly(r) {
			out = append(out, m.compactRow(bars, r, saved))
			continue
		}
		out = append(out, m.compose(m.width, seg{r.Harness, m.th.Accent}))
		out = append(out,
			m.metric(bars, "Tokens saved (estimate)", true, saved, Human(r.TokensSaved), "", m.th.OK),
			m.metric(bars, "Ran below session model", r.RoutingObserved, r.RoutedSharePct,
				pct(r.RoutedSharePct), "of "+Human(r.Billable)+" billable", m.th.Info),
			m.metric(bars, "  by Downshift rewrite", r.RoutingObserved, rewrittenPct(r),
				pct(rewrittenPct(r)), fmt.Sprintf("%s tokens, %d spawns", Human(r.RewrittenBillable), r.RewrittenSpawns), m.th.Info),
			m.metric(bars, "Premium preserved", r.PremiumObserved, r.PremiumPct,
				pct(r.PremiumPct), fmt.Sprintf("%d of %d tasks", r.NonFrontier, r.TierKnown), m.th.OK),
			m.rework(r, bars))
	}
	return out
}

// rewrittenPct is the share of the harness's billable tokens that ran below the
// session model because of a Downshift rewrite; 0 with no billable tokens.
func rewrittenPct(r Row) float64 {
	if r.Billable <= 0 {
		return 0
	}
	return float64(r.RewrittenBillable) / float64(r.Billable) * 100
}

func (m Model) metric(bars bool, label string, observed bool, share float64, value, ctx string, st lipgloss.Style) string {
	segs := []seg{{fmt.Sprintf("%-*s", labelCells, label), m.th.Bold}}
	if !observed {
		return m.compose(m.width, append(segs, seg{"unobserved", m.th.Muted})...)
	}
	if bars {
		segs = append(segs, seg{bar(share) + "  ", st})
	}
	if ctx == "" {
		return m.compose(m.width, append(segs, seg{value, m.th.Bold})...)
	}
	segs = append(segs, seg{fmt.Sprintf("%-5s", value), m.th.Bold}, seg{" " + ctx, m.th.Muted})
	return m.compose(m.width, segs...)
}

func (m Model) rework(r Row, bars bool) string {
	segs := []seg{{fmt.Sprintf("%-*s", labelCells, "Rework"), m.th.Bold}}
	if r.Ended == 0 {
		return m.compose(m.width, append(segs, seg{"unobserved", m.th.Muted})...)
	}
	pad := ""
	if bars {
		pad = strings.Repeat(" ", barCells+2)
	}
	return m.compose(m.width, append(segs, seg{pad + fmt.Sprintf("%-5s", pct(r.ReworkPct)), m.th.Bold},
		seg{fmt.Sprintf(" %d of %d ended", r.Reworked, r.Ended), m.th.Muted})...)
}

// compactOnly is a harness with Compactor savings but no routing evidence: it
// takes one line so the observed harnesses stay on the first screen.
func compactOnly(r Row) bool {
	return !r.RoutingObserved && !r.PremiumObserved && r.Ended == 0
}

func (m Model) compactRow(bars bool, r Row, saved float64) string {
	segs := []seg{{fmt.Sprintf("%-*s", labelCells, r.Harness), m.th.Accent}}
	if bars {
		segs = append(segs, seg{bar(saved) + "  ", m.th.OK})
	}
	return m.compose(m.width, append(segs, seg{fmt.Sprintf("%-5s", Human(r.TokensSaved)), m.th.Bold},
		seg{" saved · unobserved", m.th.Muted})...)
}
