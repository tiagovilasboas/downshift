// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/frame"
	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/pick"
)

func harnessItems(hs []Harness) []pick.Item {
	items := make([]pick.Item, 0, len(hs))
	for _, h := range hs {
		var hint []string
		if h.Note != "" {
			hint = append(hint, h.Note)
		}
		if !h.Detected {
			hint = append(hint, "not detected")
		}
		items = append(items, pick.Item{ID: h.ID, Label: h.Label, Hint: strings.Join(hint, " · "), Selected: h.Detected && !h.Unchecked})
	}
	return items
}

// leaveHarnesses advances, unless no harness is selected.
func (m Model) leaveHarnesses() Model {
	if len(m.harnesses.Result()) == 0 {
		m.msg = "Select at least one harness to continue."
		return m
	}
	if m.hasPresets() {
		return m.enterPreset()
	}
	m.step = stepComponents
	return m
}

var pickKeys = []frame.Key{
	{Key: "↑/↓", Help: "move"}, {Key: "space", Help: "toggle"}, {Key: "a", Help: "all"},
	{Key: "n", Help: "none"}, {Key: "enter", Help: "next"},
}

// viewPick draws a multi-select under the wizard header with the wizard's keys.
func (m Model) viewPick(p pick.MultiSelect, escHelp string) string {
	lines := strings.Split(p.View(), "\n")
	if n := len(lines); n >= 2 { // drop the list's own blank line and footer
		lines = lines[:n-2]
	}
	keys := append(append([]frame.Key(nil), pickKeys...), frame.Key{Key: "esc", Help: escHelp})
	return m.screen(strings.Join(lines, "\n"), keys)
}
