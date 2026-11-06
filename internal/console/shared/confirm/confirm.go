// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package confirm is a yes/no dialog: a title, the lines saying what will
// happen, and two choices. It is a Bubble Tea model with no program inside.
package confirm

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

// Model is the dialog state.
type Model struct {
	th     theme.Theme
	title  string
	body   []string
	yes    bool // focused choice
	done   bool
	result bool
	width  int
}

// New builds a dialog. When destructive is true the focus starts on No.
func New(th theme.Theme, title string, body []string, destructive bool) Model {
	return Model{th: th, title: title, body: body, yes: !destructive}
}

// Done reports whether the user has answered.
func (m Model) Done() bool { return m.done }

// Yes reports whether the answer was yes. It is false until Done.
func (m Model) Yes() bool {
	if !m.done {
		return false
	}
	return m.result
}

// Init has nothing to start.
func (m Model) Init() tea.Cmd { return nil }

// Update handles keys and window size.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tea.KeyPressMsg:
		if m.done {
			return m, nil
		}
		switch msg.String() {
		case "left", "right", "tab", "shift+tab", "h", "l":
			m.yes = !m.yes
		case "y":
			m.answer(true)
		case "n", "esc":
			m.answer(false)
		case "enter":
			m.answer(m.yes)
		}
	}
	return m, nil
}

func (m *Model) answer(yes bool) {
	m.done = true
	m.result = yes
	m.yes = yes
}

// View draws the dialog.
func (m Model) View() string {
	var b strings.Builder
	title := m.th.Bold
	if m.th.NoColor {
		title = lipgloss.NewStyle()
	}
	b.WriteString(m.clip(title, m.title) + "\n\n")
	for _, l := range m.body {
		b.WriteString(m.clip(m.th.Muted, "  "+l) + "\n")
	}
	if len(m.body) > 0 {
		b.WriteString("\n")
	}
	b.WriteString(m.choice("Yes", m.yes) + " " + m.choice("No", !m.yes) + "\n\n")
	b.WriteString(frame.Footer([]frame.Key{
		{Key: "←/→", Help: "switch"}, {Key: "y/n", Help: "answer"},
		{Key: "enter", Help: "confirm"}, {Key: "esc", Help: "no"},
	}, m.width))
	return b.String()
}

// clip renders s cut to the width in cells; a zero width means no limit.
func (m Model) clip(st lipgloss.Style, s string) string {
	if m.width != 0 {
		st = st.MaxWidth(m.width)
	}
	return st.Render(s)
}

func (m Model) choice(label string, on bool) string {
	if on {
		text := theme.SymPointer + " " + label + " "
		if m.th.NoColor {
			return text
		}
		return m.th.Selected.Render(text)
	}
	return "  " + label + " "
}
