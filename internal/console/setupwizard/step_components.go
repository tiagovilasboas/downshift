// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import "github.com/tiagovilasboas/downshift/internal/console/shared/pick"

func componentItems(cs []Component) []pick.Item {
	items := make([]pick.Item, 0, len(cs))
	for _, c := range cs {
		hint := c.Hint
		if c.Destructive {
			hint = "destructive: " + c.Hint
		}
		items = append(items, pick.Item{ID: c.ID, Label: c.Label, Hint: hint, Selected: c.Default && !c.Destructive,
			Disabled: c.Disabled, DisabledReason: c.DisabledReason})
	}
	return items
}

// destructiveChosen lists the selected components marked destructive.
func (m Model) destructiveChosen() []Component {
	chosen := map[string]bool{}
	for _, id := range m.components.Result() {
		chosen[id] = true
	}
	var out []Component
	for _, c := range m.o.Components {
		if c.Destructive && chosen[c.ID] {
			out = append(out, c)
		}
	}
	return out
}
