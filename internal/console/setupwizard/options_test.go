// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setupwizard

import "testing"

func TestDisabledComponentAndUncheckedHarness(t *testing.T) {
	cs := componentItems([]Component{{ID: "hooks", Label: "Hooks", Default: true, Disabled: true, DisabledReason: "always on"}})
	if !cs[0].Disabled || cs[0].DisabledReason != "always on" {
		t.Errorf("disabled component = %+v", cs[0])
	}
	hs := harnessItems([]Harness{{ID: "a", Detected: true, Unchecked: true}, {ID: "b", Detected: true}})
	if hs[0].Selected || !hs[1].Selected {
		t.Errorf("harness selection = %+v", hs)
	}
	if hs[0].Hint != "" {
		t.Errorf("a detected harness must not say not detected: %q", hs[0].Hint)
	}
}
