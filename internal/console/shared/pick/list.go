// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package pick

import (
	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
)

// List is a single-choice picker.
type List struct{ s state }

// NewList returns a List over items, focused on the first row.
func NewList(items []Item) List { return List{s: newState(items)} }

// SetSize sets the width and the number of visible item rows (zero or less
// shows every row).
func (l *List) SetSize(width, height int) { l.s.setSize(width, height) }

var listKeys = []frame.Key{
	{Key: "↑/↓", Help: "move"},
	{Key: "enter", Help: "choose"},
	{Key: "esc", Help: "cancel"},
}

// Update handles key presses and window size messages.
func (l List) Update(msg tea.Msg) (List, tea.Cmd) {
	key, ok := l.s.handle(msg)
	if !ok || key != "enter" {
		return l, nil
	}
	if it, found := l.s.current(); found && !it.Disabled {
		l.s.done = true
	}
	return l, nil
}

// View renders the visible rows and the key help.
func (l List) View() string { return l.s.render(listKeys, func(Item) string { return "" }) }

// Done reports whether an item was confirmed.
func (l List) Done() bool { return l.s.done }

// Cancelled reports whether the user pressed esc.
func (l List) Cancelled() bool { return l.s.cancelled }

// Result returns the confirmed item ID, or "" before confirmation.
func (l List) Result() string {
	if it, ok := l.s.current(); ok && l.s.done {
		return it.ID
	}
	return ""
}
