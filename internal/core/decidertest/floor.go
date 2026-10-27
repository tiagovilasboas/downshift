// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decidertest

import "github.com/tiagovilasboas/downshift/internal/core"

// Floor returns a Decider that asks d and then applies the operator tier
// floor the shared runner installed for the spawn's project, the same seam
// every harness reaches (core.TierFloorHook). The floor can only raise: with
// no project policy, or one that fails open, the decision is d's.
func Floor(d core.Decider) core.Decider {
	return core.DeciderFunc(func(req core.Request) core.Decision {
		out := d.Decide(req)
		if core.TierFloorHook != nil {
			core.TierFloorHook(&out, req.CurrentModelID, req.Resolver)
		}
		return out
	})
}
