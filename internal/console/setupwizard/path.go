// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import tea "charm.land/bubbletea/v2"

// path is the sequence of steps shown for the current choices. Before a
// preset is picked it is the longest one (the custom path).
func (m Model) path() []step {
	p := []step{stepHarnesses}
	if m.hasPresets() {
		p = append(p, stepPreset)
	}
	if m.custom() {
		p = append(p, stepComponents)
	}
	if m.choiceInPath() {
		p = append(p, stepChoice)
	}
	return append(p, stepReview, stepConfirm)
}

// total is the number of steps on the current path.
func (m Model) total() int { return len(m.path()) }

// ordinal is the 1-based position of the current step on the path.
func (m Model) ordinal() int {
	for i, s := range m.path() {
		if s == m.step {
			return i + 1
		}
	}
	return 1
}

// back moves to the step before the current one on the path. Leaving the
// review drops a preview still in flight.
func (m Model) back() (Model, tea.Cmd) {
	p := m.path()
	prev := stepHarnesses
	for i, s := range p {
		if s == m.step && i > 0 {
			prev = p[i-1]
		}
	}
	if m.step == stepReview {
		m.gen++
		m.loading = false
	}
	switch prev {
	case stepPreset:
		return m.enterPreset(), nil
	case stepChoice:
		return m.enterChoice(), nil
	}
	m.step = prev
	return m, nil
}

// afterComponents is the step that follows the component list.
func (m Model) afterComponents() (Model, tea.Cmd) {
	if m.choiceInPath() {
		return m.enterChoice(), nil
	}
	return m.startReview()
}
