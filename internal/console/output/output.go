// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package output shows what a command printed, in a scrollable pane, with its
// exit code. It is where every command the palette runs ends up.
package output

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift/internal/console/shared/theme"
)

// Hints are the pane's key hints.
const Hints = "↑↓ scroll  g/G top/end  r run again  esc back"

// RunningHints are the key hints while the command runs.
const RunningHints = "esc cancel"

// RerunMsg asks the app to run the same command again.
type RerunMsg struct{ Args []string }

// ClosedMsg is sent when the user leaves the pane.
type ClosedMsg struct{}

// Model holds one run's output and the scroll position.
type Model struct {
	args    []string
	running bool
	lines   []string
	res     command.Result
	top     int
	frame   int
}

// Running returns a pane waiting for args to finish.
func Running(args []string) Model { return Model{args: args, running: true} }

// Done returns a pane showing res.
func Done(res command.Result) Model {
	text := strings.TrimRight(res.Stdout, "\n")
	if e := strings.TrimRight(res.Stderr, "\n"); e != "" {
		if text != "" {
			text += "\n"
		}
		text += e
	}
	return Model{args: res.Args, res: res, lines: strings.Split(text, "\n")}
}

// IsRunning reports whether the command is still running.
func (m Model) IsRunning() bool { return m.running }

// Advance moves the spinner one frame.
func (m Model) Advance() Model {
	m.frame++
	return m
}

// bodyRows is how many output lines fit in rows: the header line takes one.
func bodyRows(rows int) int { return max(rows-1, 1) }

// Update handles scrolling and leaving.
func (m Model) Update(msg tea.Msg, rows int) (Model, tea.Cmd) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok || m.running {
		return m, nil
	}
	rows = bodyRows(rows)
	maxTop := max(len(m.lines)-rows, 0)
	switch key.String() {
	case "esc", "q":
		return m, func() tea.Msg { return ClosedMsg{} }
	case "r":
		args := m.args
		return m, func() tea.Msg { return RerunMsg{Args: args} }
	case "up", "k":
		m.top = max(m.top-1, 0)
	case "down", "j":
		m.top = min(m.top+1, maxTop)
	case "pgup":
		m.top = max(m.top-rows, 0)
	case "pgdown", "space":
		m.top = min(m.top+rows, maxTop)
	case "g":
		m.top = 0
	case "G":
		m.top = maxTop
	}
	return m, nil
}

// position is the "x–y of z" scroll indicator, empty when everything fits.
func (m Model) position(body int) string {
	if len(m.lines) <= body {
		return ""
	}
	return fmt.Sprintf("%d–%d of %d", m.top+1, min(m.top+body, len(m.lines)), len(m.lines))
}

// View renders the pane in rows lines, each clipped to width cells.
func (m Model) View(t theme.Theme, rows, width int) string {
	title := t.Bold.Render(frame.Clip("downshift "+strings.Join(m.args, " "), max(width-3, 0)))
	if m.running {
		return fmt.Sprintf("\n %s\n\n %s running…  %s\n", title,
			t.Accent.Render(frame.Spinner(m.frame)), t.Muted.Render("esc cancels"))
	}
	body := bodyRows(rows)
	var b strings.Builder
	head := status(t, m.res)
	if pos := m.position(body); pos != "" {
		head += "  " + t.Muted.Render(pos)
	}
	fmt.Fprintf(&b, " %s  %s\n", title, head)
	for i := m.top; i < len(m.lines) && i < m.top+body; i++ {
		b.WriteString(" " + frame.Clip(m.lines[i], max(width-2, 0)) + "\n")
	}
	if len(m.lines) == 1 && m.lines[0] == "" {
		b.WriteString(" " + t.Muted.Render(theme.SymMuted+" the command printed nothing") + "\n")
	}
	return b.String()
}

func status(t theme.Theme, r command.Result) string {
	switch {
	case errors.Is(r.Err, context.Canceled):
		return t.Warn.Render(theme.SymWarn + " canceled")
	case r.Err != nil:
		return t.Fail.Render(theme.SymFail + " " + r.Err.Error())
	case r.Code != 0:
		return t.Fail.Render(fmt.Sprintf("%s exit %d", theme.SymFail, r.Code))
	}
	return t.OK.Render(theme.SymOK + " exit 0")
}
