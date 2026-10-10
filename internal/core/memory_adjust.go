// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// MemoryAdjustHook applies local tier memory after guardrails. It is registered
// from routeadapt at process init so core stays free of routingv2 imports.
var MemoryAdjustHook func(prompt string, d *Decision, res Resolver)

func applyAdaptMemory(prompt string, d *Decision, res Resolver) {
	if MemoryAdjustHook != nil {
		MemoryAdjustHook(prompt, d, res)
	}
}
