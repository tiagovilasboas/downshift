// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Re-routing the model Route itself chose must be a no-op: a second hook
// (or this one run twice) sees the rewritten model and leaves it alone.
func TestRoute_IsFixedPointOnItsOwnOutput(t *testing.T) {
	for _, prompt := range []string{
		"rename the userId variable to userIdentifier",
		"design a distributed consensus protocol with failure analysis",
	} {
		first := core.Route(prompt, "claude-code", catID("claude-code", core.TierFrontier), cat)
		second := core.Route(prompt, "claude-code", first.Model.ID, cat)
		if second.Verdict != core.VerdictOK {
			t.Errorf("%q: second pass verdict = %s, want OK", prompt, second.Verdict)
		}
		if second.Model.ID != first.Model.ID {
			t.Errorf("%q: model changed %s -> %s", prompt, first.Model.ID, second.Model.ID)
		}
	}
}
