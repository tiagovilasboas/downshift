// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core

// HarnessCapabilities describes what a harness's hook protocol can carry.
// Routing policy is shared; adapters only declare what their wire format
// supports. Add a new harness by adding a named var below — no adapter code
// needs to change.
type HarnessCapabilities struct {
	CanRewriteModel bool // hook output can replace the subagent model ID
	CanApplyEffort  bool // hook output can set reasoning effort level
}

// Per-harness capability table. Adapters import these instead of
// declaring HarnessCapabilities inline.
var (
	// ClaudeCodeCaps — PreToolUse hook honors updatedInput.model.
	ClaudeCodeCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false}

	// CursorCaps — preToolUse hook accepts updated_input.model.
	// Note: model rewrite is silently ignored on free/legacy plans.
	CursorCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false}

	// CodexCaps — PreToolUse hook accepts updatedInput.model + reasoning_effort.
	CodexCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: true}

	// KiroCrewCaps — KiroCrew's preToolUse hook is policy-only: its contract is
	// exit 0 (allow) / exit 2 (block + stderr to the LLM), with NO updated_input
	// rewrite path. So the model cannot be replaced in place; the adapter blocks
	// a mismatched spawn and instructs the agent to respawn at the right tier.
	KiroCrewCaps = HarnessCapabilities{CanRewriteModel: false, CanApplyEffort: false}
)

// RewritePlan is the concrete set of protocol actions an adapter should take.
type RewritePlan struct {
	Model            Model
	RewriteModel     bool // write Model.ID into the hook output
	ApplyEffort      bool // write effort value into the hook output
	PreserveExplicit bool // current model is explicit_only; skip all rewrites
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
