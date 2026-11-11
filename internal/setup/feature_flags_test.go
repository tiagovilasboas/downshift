// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import "testing"

// Every declared flag must be complete: doctor prints Line() and Purpose as
// the fix and the reason.
func TestTargetFeatureFlagsAreComplete(t *testing.T) {
	for _, tg := range Targets {
		for _, f := range tg.FeatureFlags {
			if f.File == "" || f.Table == "" || f.Key == "" || f.Purpose == "" {
				t.Errorf("%s: incomplete feature flag %+v", tg.Name, f)
			}
		}
	}
	if got := (FeatureFlag{Table: "features", Key: "hooks"}).Line(); got != "[features] hooks = true" {
		t.Errorf("Line() = %q", got)
	}
}
