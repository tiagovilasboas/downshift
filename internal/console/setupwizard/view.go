// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
)

// EquivalentCommand renders the non-interactive command for a selection.
// Flag mapping belongs to the caller, which passes it as toArgs.
func EquivalentCommand(s Selection, toArgs func(Selection) []string) string {
	return strings.TrimSpace("downshift setup " + strings.Join(toArgs(s), " "))
}

// total is the number of steps shown: the optional question adds one.
func (m Model) total() int {
	if m.hasChoice() {
		return 5
	}
	return 4
}

// ordinal is the 1-based position of the current step as shown. The question
// sits after the components; review and confirm move down by one with it.
func (m Model) ordinal() int {
	switch {
	case m.step == stepChoice:
		return 3
	case m.hasChoice() && m.step >= stepReview:
		return int(m.step) + 2
	}
	return int(m.step) + 1
}

func (m Model) header() string {
	h := frame.Header("Downshift", fmt.Sprintf("Setup · step %d/%d", m.ordinal(), m.total()), m.width)
	if m.th.NoColor {
		return h
	}
	return m.th.Bold.Render(h)
}

// screen lays out the header, a body and the key help; an inline message sits above the keys.
func (m Model) screen(body string, keys []frame.Key) string {
	var b strings.Builder
	b.WriteString(m.header() + "\n\n" + body + "\n")
	if m.msg != "" {
		b.WriteString(m.th.Warn.Render("! "+m.msg) + "\n")
	}
	b.WriteString("\n" + m.th.Muted.Render(frame.Footer(keys, m.width)))
	return b.String()
}

// View draws the current step.
func (m Model) View() string {
	switch m.step {
	case stepHarnesses:
		return m.viewPick(m.harnesses, "cancel")
	case stepComponents:
		return m.viewPick(m.components, "back")
	case stepChoice:
		return m.viewChoice()
	case stepReview:
		return m.viewReview()
	}
	return m.header() + "\n\n" + m.confirm.View()
}
