// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package frame draws the parts every screen shares: the tab bar on top, a
// rule between sections, and the key hints at the bottom.
package frame

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

// Tabs renders the tab bar with the active tab highlighted. Below 80 columns
// the names collapse to their numbers.
func Tabs(t theme.Theme, names []string, active, width int) string {
	parts := []string{t.Accent.Render(" downshift ")}
	for i, n := range names {
		label := fmt.Sprintf(" %d %s ", i+1, n)
		if width > 0 && width < 80 {
			label = fmt.Sprintf(" %d ", i+1)
		}
		if i == active {
			if t.NoColor {
				label = "[" + strings.TrimSpace(label) + "]"
			}
			label = t.Selected.Render(label)
		}
		parts = append(parts, label)
	}
	parts = append(parts, t.Muted.Render("  : commands"))
	return strings.Join(parts, " ")
}

// Rule is a full-width thin line.
func Rule(t theme.Theme, width int) string {
	if width <= 0 {
		width = 80
	}
	return t.Muted.Render(strings.Repeat("─", width))
}

// Hints renders the bottom key line, with right-aligned context. The hints are
// cut with the same cell-safe rule as Footer so the line never exceeds width.
func Hints(t theme.Theme, hints, right string, width int) string {
	if width > 0 {
		room := width - lipgloss.Width(right) - 3
		if room < 0 {
			room = 0
		}
		hints = clip(hints, room)
	}
	gap := width - lipgloss.Width(hints) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	return t.Muted.Render(" " + hints + strings.Repeat(" ", gap) + right)
}

// Screen stacks header, body and footer, padding the body so the footer sits
// on the last line.
func Screen(header, body, footer string, height int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	room := height - 4
	if room > 0 && len(lines) > room {
		lines = lines[:room]
	}
	for room > 0 && len(lines) < room {
		lines = append(lines, "")
	}
	return header + "\n" + strings.Join(lines, "\n") + "\n" + footer
}
