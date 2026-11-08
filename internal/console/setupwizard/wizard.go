// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package setupwizard is the interactive setup flow: pick harnesses, pick
// components, review the dry-run diff, confirm. It is a pure Bubble Tea model;
// the caller injects the preview and applies the result. No setup logic lives
// here, and nothing here runs a tea.Program.
package setupwizard

import (
	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/confirm"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/diffview"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/pick"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

// Harness is a coding harness the wizard can set up. A detected one starts selected.
type Harness struct {
	ID, Label string
	Detected  bool
	// Unchecked starts a detected harness unselected (a name on the command line chose others).
	Unchecked bool
	Note      string
}

// Component is one part of the setup. Destructive components start unselected.
type Component struct {
	ID, Label, Hint string
	Default         bool
	Destructive     bool
	// Disabled shows the component as fixed (not selectable); Reason says why.
	Disabled       bool
	DisabledReason string
}

// Selection is what the user chose.
type Selection struct {
	Harnesses, Components []string
	// Choice is the answer to Options.Choice; empty when there is no question.
	Choice string
}

// Options injects the data and the dry-run port.
type Options struct {
	Harnesses  []Harness
	Components []Component
	// Preview computes the dry-run diff for a selection. It runs as a command.
	Preview func(Selection) ([]diffview.File, error)
	// Choice is an optional question after the components (nil: not asked).
	Choice *Choice
}

type step int

const (
	stepHarnesses step = iota
	stepComponents
	stepReview
	stepConfirm
	stepChoice // after the components when Options.Choice is set; shown in that place
)

const (
	defaultWidth  = 80
	defaultHeight = 24
)

// Model is the wizard.
type Model struct {
	th            theme.Theme
	o             Options
	step          step
	harnesses     pick.MultiSelect
	components    pick.MultiSelect
	choice        pick.List
	answer        string // the confirmed Choice item
	diff          diffview.Model
	files         []diffview.File
	confirm       confirm.Model
	loading       bool
	ticking       bool
	tick          int
	gen           int // identifies the preview in flight; older results are dropped
	err           error
	msg           string // inline warning for the current step
	done          bool
	cancelled     bool
	width, height int
}

type previewMsg struct {
	gen   int
	files []diffview.File
	err   error
}

// New returns the wizard at its first step.
func New(th theme.Theme, o Options) Model {
	m := Model{th: th, o: o, width: defaultWidth, height: defaultHeight}
	m.harnesses = pick.NewMultiSelect(harnessItems(o.Harnesses))
	m.components = pick.NewMultiSelect(componentItems(o.Components))
	m.resize(m.width, m.height)
	return m
}

// Done reports that the user confirmed the changes.
func (m Model) Done() bool { return m.done }

// Cancelled reports that the user left the wizard.
func (m Model) Cancelled() bool { return m.cancelled }

// Selection returns the current choice.
func (m Model) Selection() Selection {
	return Selection{Harnesses: m.harnesses.Result(), Components: m.components.Result(), Choice: m.answer}
}

// Init has nothing to start.
func (m Model) Init() tea.Cmd { return nil }

// Update routes the message to the current step.
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.done || m.cancelled {
		return m, nil
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m.forward(msg)
	case frame.TickMsg:
		if !m.loading {
			m.ticking = false
			return m, nil
		}
		m.tick++
		return m, frame.Tick()
	case previewMsg:
		return m.previewed(msg), nil
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m *Model) resize(w, h int) {
	m.width, m.height = w, h
	rows := h - 4 // header, blank line, blank line, footer
	m.harnesses.SetSize(w, rows)
	m.components.SetSize(w, rows)
	m.choice.SetSize(w, 0)
}

func (m Model) key(k tea.KeyPressMsg) (Model, tea.Cmd) {
	s := k.String()
	if s == "ctrl+c" {
		m.cancelled = true
		return m, nil
	}
	m.msg = ""
	back := s == "esc" || s == "backspace"
	switch m.step {
	case stepHarnesses:
		switch s {
		case "esc":
			m.cancelled = true
			return m, nil
		case "enter":
			return m.leaveHarnesses(), nil
		}
	case stepComponents:
		switch {
		case back:
			m.step = stepHarnesses
			return m, nil
		case s == "enter":
			if m.hasChoice() {
				return m.enterChoice(), nil
			}
			return m.startReview()
		}
	case stepChoice:
		if back {
			m.step = stepComponents
			return m, nil
		}
	case stepReview:
		switch {
		case back:
			m.gen++ // drop a preview still in flight
			m.loading = false
			m.step = stepComponents
			if m.hasChoice() {
				m = m.enterChoice()
			}
			return m, nil
		case s == "enter":
			return m.leaveReview(), nil
		}
	case stepConfirm:
		if back {
			m.step = stepReview
			return m, nil
		}
	}
	return m.forward(k)
}

// forward hands a message to the component that owns the current step.
func (m Model) forward(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.step {
	case stepHarnesses:
		m.harnesses, cmd = m.harnesses.Update(msg)
	case stepComponents:
		m.components, cmd = m.components.Update(msg)
	case stepChoice:
		m.choice, cmd = m.choice.Update(msg)
		if m.choice.Done() {
			m.answer = m.choice.Result()
			return m.startReview()
		}
	case stepReview:
		m.diff, cmd = m.diff.Update(msg)
	case stepConfirm:
		m.confirm, cmd = m.confirm.Update(msg)
		if m.confirm.Done() {
			if m.confirm.Yes() {
				m.done = true
			} else {
				m.step = stepReview
			}
		}
	}
	return m, cmd
}
