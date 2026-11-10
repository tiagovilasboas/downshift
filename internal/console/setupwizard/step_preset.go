// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/pick"
)

// Preset is a ready-made set of components, asked right after the harnesses.
// A Custom preset opens the component list; any other one selects its
// Components and skips the list. AskChoice keeps the optional question
// (Options.Choice) on a non-custom path. The first preset is the one Enter
// picks, so the caller puts the recommended one there.
type Preset struct {
	ID, Label, Hint string
	Components      []string
	Custom          bool
	AskChoice       bool
}

func (m Model) hasPresets() bool { return len(m.o.Presets) > 0 }

// chosenPreset is the preset picked, or nil before the pick (or without presets).
func (m Model) chosenPreset() *Preset {
	if m.presetIdx < 0 || m.presetIdx >= len(m.o.Presets) {
		return nil
	}
	return &m.o.Presets[m.presetIdx]
}

// custom reports whether the component list is part of the path: always
// without presets, and before a preset is picked (the longest path).
func (m Model) custom() bool {
	p := m.chosenPreset()
	return !m.hasPresets() || p == nil || p.Custom
}

// choiceInPath reports whether the optional question is asked on this path.
func (m Model) choiceInPath() bool {
	if !m.hasChoice() {
		return false
	}
	p := m.chosenPreset()
	return m.custom() || (p != nil && p.AskChoice)
}

// enterPreset shows the presets with the cursor on the last one picked.
func (m Model) enterPreset() Model {
	m.step = stepPreset
	items := make([]pick.Item, 0, len(m.o.Presets))
	for _, p := range m.o.Presets {
		items = append(items, pick.Item{ID: p.ID, Label: p.Label, Hint: p.Hint})
	}
	m.preset = pick.NewList(items)
	m.preset.SetSize(m.width, 0)
	for i := 0; i < m.presetIdx; i++ {
		m.preset, _ = m.preset.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	}
	return m
}

// applyPreset records the pick and moves on: the component list for a custom
// preset, else the preset's components, then the question or the review.
func (m Model) applyPreset(id string) (Model, tea.Cmd) {
	for i, p := range m.o.Presets {
		if p.ID == id {
			m.presetIdx = i
		}
	}
	p := m.chosenPreset()
	if p == nil || p.Custom {
		m.step = stepComponents
		return m, nil
	}
	m.components = pick.NewMultiSelect(presetItems(m.o.Components, p.Components))
	m.components.SetSize(m.width, m.height-4)
	m.answer = ""
	if m.choiceInPath() {
		return m.enterChoice(), nil
	}
	return m.startReview()
}

// presetItems selects exactly ids, keeping fixed components as they are and
// never selecting a destructive one.
func presetItems(cs []Component, ids []string) []pick.Item {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	items := componentItems(cs)
	for i, c := range cs {
		if !c.Disabled {
			items[i].Selected = want[c.ID] && !c.Destructive
		}
	}
	return items
}

func (m Model) viewPreset() string {
	lines := strings.Split(m.preset.View(), "\n")
	if n := len(lines); n >= 2 { // drop the list's own blank line and footer
		lines = lines[:n-2]
	}
	body := m.th.Bold.Render("How much do you want to install?") + "\n" + strings.Join(lines, "\n")
	return m.screen(body, []frame.Key{{Key: "↑/↓", Help: "move"}, {Key: "enter", Help: "choose"}, {Key: "esc", Help: "back"}})
}
