// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/console/shared/confirm"
)

// newConfirm builds the final dialog; it defaults to No when anything destructive is chosen.
func newConfirm(m Model) confirm.Model {
	sel := m.Selection()
	noun := "files"
	if len(m.files) == 1 {
		noun = "file"
	}
	body := []string{
		fmt.Sprintf("%d %s to write", len(m.files), noun),
		"harnesses: " + strings.Join(sel.Harnesses, ", "),
		"components: " + strings.Join(sel.Components, ", "),
	}
	danger := m.destructiveChosen()
	for _, c := range danger {
		body = append(body, "destructive: "+c.Label+" — "+c.Hint)
	}
	return confirm.New(m.th, "Apply these changes?", body, len(danger) > 0)
}
