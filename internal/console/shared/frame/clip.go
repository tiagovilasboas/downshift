// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package frame

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// Clip cuts plain (unstyled) text to at most width cells, ending in an
// ellipsis when it cuts. A width of zero or less means no limit.
func Clip(s string, width int) string { return clip(s, width) }

// TickMsg advances every spinner on screen by one frame.
type TickMsg struct{}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner returns the spinner glyph for animation frame i.
func Spinner(i int) string {
	n := len(spinnerFrames)
	return spinnerFrames[((i%n)+n)%n]
}

// Tick schedules the next TickMsg.
func Tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(time.Time) tea.Msg { return TickMsg{} })
}
