// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package home is the console's first screen: this session at a glance and the
// health checks that need attention, each with its fix.
package home

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

// Name is the tab label.
const Name = "Home"

// Hints are the screen's key hints.
const Hints = "r refresh  : commands  q quit"

type loadedMsg struct {
	session command.Result
	doctor  command.Result
}

// Model holds the screen state.
type Model struct {
	run     command.Runner
	loading bool
	session command.Result
	health  health
	err     error
	tick    int
}

// Loading reports whether the screen is still reading.
func (m Model) Loading() bool { return m.loading }

// Advance moves the spinner one frame.
func (m Model) Advance() Model {
	m.tick++
	return m
}

// New returns the screen, not yet loaded.
func New(run command.Runner) Model { return Model{run: run, loading: true} }

// Load reads the session brief and the doctor checks in the background.
func (m Model) Load() tea.Cmd {
	run := m.run
	return func() tea.Msg {
		ctx := context.Background()
		return loadedMsg{
			session: run.Capture(ctx, []string{"session", "--brief"}),
			doctor:  run.Capture(ctx, []string{"doctor", "--json"}),
		}
	}
}

// Update handles the screen's messages and keys.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadedMsg:
		m.loading = false
		m.session = msg.session
		m.health, m.err = parseHealth(msg.doctor)
	case tea.KeyPressMsg:
		if msg.String() == "r" && !m.loading {
			m.loading = true
			return m, m.Load() // the app starts the spinner tick
		}
	}
	return m, nil
}

// View renders the screen body.
func (m Model) View(t theme.Theme, width int) string {
	if m.loading {
		return "\n " + t.Accent.Render(frame.Spinner(m.tick)) + " reading session and health checks…"
	}
	var b strings.Builder
	b.WriteString("\n " + t.Bold.Render("This session") + "\n")
	b.WriteString(sessionBlock(t, m.session, width))
	b.WriteString("\n " + t.Bold.Render("Health") + "\n")
	b.WriteString(healthBlock(t, m.health, m.err, width))
	return b.String()
}

func sessionBlock(t theme.Theme, r command.Result, width int) string {
	if r.Err != nil {
		return fmt.Sprintf(" %s session did not load: %s\n   %s\n", t.Fail.Render(theme.SymFail),
			frame.Clip(oneLine(r.Err.Error(), width-24), max(width-24, 0)), t.Muted.Render("r try again"))
	}
	text := strings.TrimSpace(r.Stdout)
	if !r.OK() || text == "" {
		return fmt.Sprintf(" %s No subagent spawns recorded in this session yet.\n   %s\n",
			t.Muted.Render(theme.SymMuted),
			t.Muted.Render("Open a new harness session and delegate a task; it shows up here."))
	}
	return indent(text, 1, width)
}

func indent(text string, n, width int) string {
	pad := strings.Repeat(" ", n)
	var b strings.Builder
	for _, l := range strings.Split(text, "\n") {
		b.WriteString(pad + frame.Clip(l, max(width-n, 0)) + "\n")
	}
	return b.String()
}
