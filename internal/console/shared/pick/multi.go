// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package pick

import (
	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
)

var multiKeys = []frame.Key{
	{Key: "↑/↓", Help: "move"},
	{Key: "space", Help: "toggle"},
	{Key: "a", Help: "all"},
	{Key: "n", Help: "none"},
	{Key: "enter", Help: "confirm"},
	{Key: "esc", Help: "cancel"},
}

// MultiSelect is a many-choice picker with preselection.
type MultiSelect struct{ s state }

// NewMultiSelect returns a MultiSelect over items; Item.Selected preselects.
// A disabled item is locked: it keeps the Selected the caller gave it, so an
// always-on part shows [x] and an unavailable one shows [ ].
func NewMultiSelect(items []Item) MultiSelect {
	return MultiSelect{s: newState(items)}
}

// SetSize sets the width and the number of visible item rows (zero or less
// shows every row).
func (m *MultiSelect) SetSize(width, height int) { m.s.setSize(width, height) }

// Update handles key presses and window size messages.
func (m MultiSelect) Update(msg tea.Msg) (MultiSelect, tea.Cmd) {
	key, ok := m.s.handle(msg)
	if !ok {
		return m, nil
	}
	switch key {
	case "space":
		m.toggle()
	case "a":
		m.setAll(true)
	case "n":
		m.setAll(false)
	case "enter":
		m.s.done = true
	}
	return m, nil
}

// toggle and setAll copy the slice: the model is a value and callers may
// hold an earlier copy.
func (m *MultiSelect) toggle() {
	it, ok := m.s.current()
	if !ok || it.Disabled {
		return
	}
	items := append([]Item(nil), m.s.items...)
	items[m.s.cursor].Selected = !it.Selected
	m.s.items = items
}

func (m *MultiSelect) setAll(v bool) {
	items := append([]Item(nil), m.s.items...)
	for i := range items {
		if !items[i].Disabled {
			items[i].Selected = v
		}
	}
	m.s.items = items
}

// View renders the visible rows and the key help.
func (m MultiSelect) View() string {
	return m.s.render(multiKeys, func(it Item) string {
		if it.Selected {
			return "[x]"
		}
		return "[ ]"
	})
}

// Done reports whether the selection was confirmed.
func (m MultiSelect) Done() bool { return m.s.done }

// Cancelled reports whether the user pressed esc.
func (m MultiSelect) Cancelled() bool { return m.s.cancelled }

// Result returns the selected IDs in item order.
func (m MultiSelect) Result() []string {
	var ids []string
	for _, it := range m.s.items {
		if it.Selected {
			ids = append(ids, it.ID)
		}
	}
	return ids
}
