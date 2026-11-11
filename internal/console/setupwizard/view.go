// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/theme"
)

// EquivalentCommand renders the non-interactive command for a selection.
// Flag mapping belongs to the caller, which passes it as toArgs.
func EquivalentCommand(s Selection, toArgs func(Selection) []string) string {
	return strings.TrimSpace("downshift setup " + strings.Join(toArgs(s), " "))
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

// View draws the current step. Once the wizard is over it is the one line
// left in the scrollback: the choice confirmed, or nothing when cancelled
// (the caller says so).
func (m Model) View() string {
	switch {
	case m.cancelled:
		return ""
	case m.done:
		return m.doneLine()
	}
	switch m.step {
	case stepHarnesses:
		return m.viewPick(m.harnesses, "cancel")
	case stepComponents:
		return m.viewPick(m.components, "back")
	case stepChoice:
		return m.viewChoice()
	case stepPreset:
		return m.viewPreset()
	case stepReview:
		return m.viewReview()
	}
	return m.header() + "\n\n" + m.confirm.View()
}

// doneLine recaps the confirmed choice in one line.
func (m Model) doneLine() string {
	sel := m.Selection()
	what := strings.Join(sel.Components, ", ")
	if p := m.chosenPreset(); p != nil && !p.Custom {
		what = p.Label + ": " + what
	}
	return m.th.OK.Render(theme.SymOK) + " " + m.th.Bold.Render("Setting up") + " " +
		strings.Join(sel.Harnesses, ", ") + m.th.Muted.Render(" · "+what) + "\n"
}
