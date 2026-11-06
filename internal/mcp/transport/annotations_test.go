// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
)

// Every registered tool must declare its four hints here, so a new tool cannot
// ship with the wrong defaults: a client may auto-approve on them.
func TestEveryRegisteredToolPinsItsHints(t *testing.T) {
	type hints struct{ readOnly, idempotent, destructive, openWorld bool }
	want := map[string]hints{
		"delegate":  {false, false, true, true},
		"send":      {false, false, true, true},
		"steer":     {false, false, true, true},
		"release":   {false, true, true, false},
		"cancel":    {false, true, true, false},
		"collect":   {false, true, false, false},
		"diff":      {false, true, false, false},
		"retrieve":  {true, true, false, false},
		"providers": {false, true, false, false},
		"playbook":  {true, true, false, false},
		"status":    {true, true, false, false},
		"plan":      {true, true, false, false},
		"apply":     {true, true, false, false},
	}
	var all []tools.Tool
	all = append(all, tools.Info(tools.Deps{})...)
	all = append(all, tools.WorkTools(tools.Deps{})...)
	for _, tl := range all {
		a := annotations(tl)
		got := hints{a.ReadOnlyHint, a.IdempotentHint, *a.DestructiveHint, *a.OpenWorldHint}
		w, ok := want[tl.Name]
		if !ok {
			t.Errorf("tool %q is not pinned: add its four hints to this test", tl.Name)
			continue
		}
		if got != w {
			t.Errorf("tool %q hints = %+v, want %+v", tl.Name, got, w)
		}
		if tl.ReadOnly && tl.Destructive {
			t.Errorf("tool %q is both read-only and destructive", tl.Name)
		}
	}
}
