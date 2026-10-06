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
	// StrictModelName means the harness validates the model field against its
	// own names (e.g. family aliases) and rejects any other string, blocking
	// the spawn. A target with no native_name in the catalog is then not
	// written: the spawn runs unchanged.
	StrictModelName bool
}

// Per-harness capability table. Adapters import these instead of
// declaring HarnessCapabilities inline.
var (
	// ClaudeCodeCaps — PreToolUse hook honors updatedInput.model, but only
	// family names (haiku/sonnet/opus/fable); a full id fails schema validation.
	ClaudeCodeCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, StrictModelName: true}

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

	// AntigravityCaps — the Antigravity hook accepts a model override on the
	// subagent payload. The wire format carries tier aliases (flash_lite,
	// flash, pro), not catalog ids, so the adapter maps the routed tier to
	// the alias and writes it only when the session plan allows a rewrite.
	// Effort is not supported: there is no reasoning-effort field.
	AntigravityCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false}
)

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
