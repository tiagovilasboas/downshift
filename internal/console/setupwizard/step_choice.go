// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/pick"
)

// Choice is an optional single-answer question asked after the components.
// The first item is the one Enter picks, so the caller puts the safe default
// there. Without a Choice the wizard has no such step.
type Choice struct {
	ID, Title string
	Items     []ChoiceItem
}

// ChoiceItem is one answer.
type ChoiceItem struct{ ID, Label, Hint string }

func choiceItems(c *Choice) []pick.Item {
	if c == nil {
		return nil
	}
	items := make([]pick.Item, 0, len(c.Items))
	for _, it := range c.Items {
		items = append(items, pick.Item{ID: it.ID, Label: it.Label, Hint: it.Hint})
	}
	return items
}

// hasChoice reports whether the wizard asks the extra question.
func (m Model) hasChoice() bool { return m.o.Choice != nil && len(m.o.Choice.Items) > 0 }

// enterChoice shows the question, with a fresh cursor on the first answer.
func (m Model) enterChoice() Model {
	m.step = stepChoice
	m.choice = pick.NewList(choiceItems(m.o.Choice))
	m.choice.SetSize(m.width, 0)
	return m
}

func (m Model) viewChoice() string {
	lines := strings.Split(m.choice.View(), "\n")
	if n := len(lines); n >= 2 { // drop the list's own blank line and footer
		lines = lines[:n-2]
	}
	body := m.th.Bold.Render(m.o.Choice.Title) + "\n" + strings.Join(lines, "\n")
	return m.screen(body, []frame.Key{{Key: "↑/↓", Help: "move"}, {Key: "enter", Help: "next"}, {Key: "esc", Help: "back"}})
}
