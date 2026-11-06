// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import "testing"

// TestLegacyModeKeyStillLoads checks that a policy file written by the removed
// economy mode still loads: the routing loader ignores the top-level "mode"
// key and the strict tier-floor decoder accepts it, whatever its value.
func TestLegacyModeKeyStillLoads(t *testing.T) {
	for _, body := range []string{
		`{"version":1,"mode":"economy"}`,
		`{"version":1,"mode":"standard","min_tier":"mid"}`,
		`{"version":1,"mode":"economy","routing":{"critical_threshold":70}}`,
		`{"version":1,"mode":"anything"}`,
	} {
		path := projectFile(t, body)
		p, err := Load(Locations{Project: path})
		if err != nil {
			t.Errorf("%s: routing Load refused: %v", body, err)
			continue
		}
		if !p.ByClass {
			t.Errorf("%s: a legacy mode key changed the routing defaults", body)
		}
		if _, err := LoadFloor(path); err != nil {
			t.Errorf("%s: tier floor refused: %v", body, err)
		}
	}
}
