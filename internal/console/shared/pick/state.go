// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package pick

import (
	tea "charm.land/bubbletea/v2"
)

// state is the cursor, viewport and outcome shared by List and MultiSelect.
type state struct {
	items     []Item
	cursor    int
	top       int // first visible row
	width     int
	height    int // visible item rows; zero or less shows every row
	done      bool
	cancelled bool
}

func newState(items []Item) state {
	return state{items: append([]Item(nil), items...)}
}

func (s *state) setSize(width, height int) {
	s.width, s.height = width, height
	s.scrollToCursor()
}

func (s *state) rows() int {
	if s.height <= 0 || s.height > len(s.items) {
		return len(s.items)
	}
	return s.height
}

// scrollToCursor keeps the focused row inside the viewport.
func (s *state) scrollToCursor() {
	n := s.rows()
	if s.cursor < s.top {
		s.top = s.cursor
	}
	if s.cursor >= s.top+n {
		s.top = s.cursor - n + 1
	}
	if s.top < 0 {
		s.top = 0
	}
}

func (s *state) move(delta int) {
	if len(s.items) == 0 {
		return
	}
	s.cursor = min(max(s.cursor+delta, 0), len(s.items)-1)
	s.scrollToCursor()
}

// handle applies the keys both components share. It returns the key name for
// the caller's own keys and true when the key was not consumed here.
func (s *state) handle(msg tea.Msg) (string, bool) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		s.setSize(m.Width, s.height)
	case tea.KeyPressMsg:
		if s.done || s.cancelled {
			return "", false
		}
		key := m.String()
		if key == " " {
			key = "space"
		}
		switch key {
		case "up", "k":
			s.move(-1)
		case "down", "j":
			s.move(1)
		case "esc":
			s.cancelled = true
		default:
			return key, true
		}
	}
	return "", false
}

func (s *state) current() (Item, bool) {
	if s.cursor < 0 || s.cursor >= len(s.items) {
		return Item{}, false
	}
	return s.items[s.cursor], true
}
