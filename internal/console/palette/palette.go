// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package palette is the command list: type to filter, enter to run. Words
// typed after a command's name are passed to it as extra arguments.
package palette

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

// Hints are the palette's key hints.
const Hints = "↑↓ move  enter run  esc close"

// ChosenMsg is sent when the user picks a command; Args include typed extras.
type ChosenMsg struct {
	Spec command.Spec
	Args []string
}

// ClosedMsg is sent when the user closes the palette without choosing.
type ClosedMsg struct{}

// Model holds the palette state.
type Model struct {
	all    []command.Spec
	query  string
	cursor int
}

// New returns a palette over specs.
func New(specs []command.Spec) Model { return Model{all: specs} }

func (m Model) matches() []command.Spec {
	if hit := command.Filter(m.all, m.query); len(hit) > 0 {
		return hit
	}
	// "report --days=3" matches nothing as a whole; fall back to the name.
	if f := strings.Fields(m.query); len(f) > 1 {
		return command.Filter(m.all, f[0])
	}
	return nil
}

// Update handles keys while the palette is open.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	hits := m.matches()
	switch key.String() {
	case "esc":
		return m, func() tea.Msg { return ClosedMsg{} }
	case "up", "ctrl+p":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "ctrl+n":
		if m.cursor < len(hits)-1 {
			m.cursor++
		}
	case "enter":
		if len(hits) == 0 {
			return m, nil
		}
		s := hits[m.cursor]
		args := command.WithExtra(s, m.query)
		return m, func() tea.Msg { return ChosenMsg{Spec: s, Args: args} }
	case "backspace":
		if m.query != "" {
			r := []rune(m.query)
			m.query, m.cursor = string(r[:len(r)-1]), 0
		}
	default:
		if key.Text != "" {
			m.query, m.cursor = m.query+key.Text, 0
		}
	}
	return m, nil
}

// View renders the palette with at most rows entries.
func (m Model) View(t theme.Theme, rows, width int) string {
	hits := m.matches()
	var b strings.Builder
	fmt.Fprintf(&b, "\n  %s %s█\n\n", t.Accent.Render(":"), m.query)
	if len(hits) == 0 {
		fmt.Fprintf(&b, "  %s no command matches %q\n", t.Muted.Render(theme.SymMuted), m.query)
		return b.String()
	}
	start := 0
	if rows > 0 && m.cursor >= rows {
		start = m.cursor - rows + 1
	}
	for i := start; i < len(hits) && (rows <= 0 || i < start+rows); i++ {
		b.WriteString(row(t, hits[i], i == m.cursor, width) + "\n")
	}
	fmt.Fprintf(&b, "\n  %s\n", t.Muted.Render(fmt.Sprintf("%d of %d commands", len(hits), len(m.all))))
	return b.String()
}

// row draws one entry. The plain text is cut to the room left of the 3-cell
// pointer column before it is styled, so a row never exceeds width.
func row(t theme.Theme, s command.Spec, selected bool, width int) string {
	kind := "read"
	if s.Writes {
		kind = "writes"
	}
	if s.Terminal {
		kind += " · terminal"
	}
	line := fmt.Sprintf("%-28s %-58s %s", s.Name(), trim(s.Summary, 58), kind)
	if width > 0 {
		line = frame.Clip(line, max(width-3, 0))
	}
	if selected {
		return " " + t.Accent.Render(theme.SymPointer) + " " + t.Selected.Render(line)
	}
	// Colour the kind label by its position, never a "writes" in the name or summary.
	if at := 28 + 1 + 58 + 1; s.Writes && len(line) >= at+len("writes") && line[at:at+len("writes")] == "writes" {
		return "   " + line[:at] + t.Warn.Render("writes") + line[at+len("writes"):]
	}
	return "   " + line
}

func trim(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}
