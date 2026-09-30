// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package decisionintelligence

// JevAdapter is an optional extension point for a future Jev-backed semantic
// adviser. It intentionally has no dependency, network call, account, or API
// key. The default adapter is disabled and Evaluate remains deterministic.
//
// Any future implementation may only add advisory signals; hard gates retain
// final authority in the deterministic routing policy.
type JevAdapter struct {
	Enabled bool
}

// Available reports whether an external Jev integration is intentionally
// enabled. It does not attempt discovery or perform I/O.
func (a JevAdapter) Available() bool {
	return a.Enabled
}
