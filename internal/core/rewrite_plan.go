// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// RewritePlan is the concrete set of protocol actions an adapter should take.
type RewritePlan struct {
	Model        Model
	RewriteModel bool // write WriteName into the hook output
	// WriteName is the exact string to write when RewriteModel is true. Session
	// and ownership checks still use Model.ID.
	WriteName        string
	ApplyEffort      bool // write effort value into the hook output
	PreserveExplicit bool // current model is explicit_only; skip all rewrites
	// HoldForeign means the requested id is not in this harness's session
	// or exact catalog. Adapters must not write a model.
	HoldForeign bool
	// CreditHeld means the credit gate refused the rewrite: a present source
	// or required mode left no writable target. Foreign, explicit-preserve,
	// and risk holds do not set it.
	CreditHeld bool
	// Session is the list the plan was made against, including one inferred
	// from the current model. Adapters pass it to CanWriteSessionID so the
	// write guard sees the same list as the planner.
	Session SessionList
}

// Plan translates a harness-agnostic Decision into protocol actions.
// Without a session list it does not rewrite: the catalog is not an allowlist.
// Adapters that know the session call PlanForSession instead.
func (d Decision) Plan(c HarnessCapabilities, r ...Resolver) RewritePlan {
	var res Resolver
	if len(r) > 0 && r[0] != nil {
		res = r[0]
	}
	return d.PlanForSession(c, res, UnknownSession())
}
