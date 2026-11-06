// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Key is one key binding with its help text, for example {"enter", "confirm"}.
type Key struct {
	Key, Help string
}

const ellipsis = "…"

// Footer renders the keys as one "key help" line, unstyled, cut to width cells
// with an ellipsis when it does not fit. A width of zero or less means no limit.
func Footer(keys []Key, width int) string {
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, strings.TrimSpace(k.Key+" "+k.Help))
	}
	return clip(strings.Join(parts, "  "), width)
}

// Header renders the screen title with the step (for example "2/4") right
// aligned, unstyled, on one line cut to width cells.
func Header(title, step string, width int) string {
	if step == "" {
		return clip(title, width)
	}
	gap := width - lipgloss.Width(title) - lipgloss.Width(step)
	if width <= 0 || gap < 2 {
		return clip(title+"  "+step, width)
	}
	return title + strings.Repeat(" ", gap) + step
}

// clip cuts s to at most width terminal cells, ending in an ellipsis when it
// cuts. It measures by cell width, so wide runes are never split.
func clip(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	if width == 1 {
		return ellipsis
	}
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w > width-1 {
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + ellipsis
}
