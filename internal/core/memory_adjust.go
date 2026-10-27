// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// MemoryAdjustHook applies local tier memory after guardrails. No package in
// this binary registers it (routeadapt was removed), so the step is a no-op.
var MemoryAdjustHook func(prompt string, d *Decision, res Resolver)

func applyAdaptMemory(prompt string, d *Decision, res Resolver) {
	if MemoryAdjustHook != nil {
		MemoryAdjustHook(prompt, d, res)
	}
}
