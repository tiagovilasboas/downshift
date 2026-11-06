// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package pick

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

// render draws the visible rows and the footer. marker returns the row's
// check box ("" for none).
func (s *state) render(keys []frame.Key, marker func(Item) string) string {
	th := theme.New()
	pointer := ">"
	if !th.NoColor {
		pointer = theme.SymPointer
	}
	var lines []string
	end := min(s.top+s.rows(), len(s.items))
	for i := s.top; i < end; i++ {
		lines = append(lines, s.row(th, pointer, i, marker))
	}
	lines = append(lines, "", th.Muted.Render(frame.Footer(keys, s.width)))
	return strings.Join(lines, "\n")
}

func (s *state) row(th theme.Theme, pointer string, i int, marker func(Item) string) string {
	it := s.items[i]
	prefix := "  "
	if i == s.cursor {
		prefix = pointer + " "
	}
	if m := marker(it); m != "" {
		prefix += m + " "
	}
	note := it.Hint
	if it.Disabled {
		note = it.DisabledReason
		if note == "" {
			note = "unavailable"
		}
		note = "(" + note + ")"
	}
	plain := prefix + it.Label
	if note != "" {
		plain += "  " + note
	}
	if s.width > 0 && lipgloss.Width(plain) > s.width {
		return th.Muted.Render(cut(plain, s.width))
	}
	label := it.Label
	switch {
	case it.Disabled:
		label = th.Muted.Render(label)
	case i == s.cursor:
		label = th.Accent.Render(label)
	}
	out := prefix + label
	if note != "" {
		out += "  " + th.Muted.Render(note)
	}
	return out
}

// cut trims s to width cells, ending in an ellipsis; it never splits a rune.
func cut(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
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
	return b.String() + "…"
}
