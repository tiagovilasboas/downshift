// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "github.com/tiagovilasboas/downshift/internal/quota"

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
	// ModelOverrideEnv names an environment variable that, per a source other
	// than Downshift's own evidence, can make the harness ignore a hook's model
	// rewrite. Empty means none is known, which is not the same as none exists.
	ModelOverrideEnv string
	// NativeSessionEnv names the environment variable holding the path of the
	// harness's native session/usage export. Empty means the harness has none.
	NativeSessionEnv string
	// NativeUsageSource is the quota.ParseNative source name for that export.
	NativeUsageSource string
	// TranscriptQuota means quota can be collected from an explicit local
	// transcript (`quota collect --transcript`).
	TranscriptQuota bool
	// SingleModelReasoning means the harness runs one model and exposes only
	// reasoning effort as a knob, so `try` reports an effort, not a model.
	SingleModelReasoning bool
	// InferSession means the current model proves access to the catalog tiers
	// below it, so an unknown session may be inferred from it (see
	// InferSessionFromCurrent). False where quota pools or tier aliases make
	// that unsafe: the session stays unknown and nothing is rewritten.
	InferSession bool
	// CanShowChildLine means the hook protocol can show the developer one line
	// of text when a subagent ends (U2). Verified per harness; false until a
	// harness is shown to display it. The line is computed either way.
	CanShowChildLine bool
	// SessionEnv names the environment variable that holds the harness's
	// current session id inside a command the developer runs from the harness
	// (U3, `downshift session`). Empty means the harness exports none, and the
	// command falls back to the most recent session in the event log.
	SessionEnv string
	// ChildEndEvent is the hook event name that carries the end of a child
	// spawn (N5). Empty means no such event is known; the shared end runner
	// then records nothing for the harness. Setup wires the end hook from this.
	ChildEndEvent string
	// UsageSource says where the child's token usage comes from at that event:
	// UsagePayload, UsageTranscript or UsageNone (also the empty value).
	UsageSource string
	// HookEntries lists the known spawn-time entry points that can call the
	// shared router for this harness (for example "settings-hook" and "mod").
	// Empty means one unnamed path. When more than one is listed, doctor can
	// warn if the same tool_use_hash carries decisions from more than one.
	HookEntries []string
}

// UsageSource values for HarnessCapabilities.UsageSource.
const (
	UsagePayload    = "payload"
	UsageTranscript = "transcript"
	UsageNone       = "none"
)

// Per-harness capability table. Adapters import these instead of
// declaring HarnessCapabilities inline.
var (
	// ClaudeCodeCaps — PreToolUse hook honors updatedInput.model, but only
	// family names (haiku/sonnet/opus/fable); a full id fails schema validation.
	ClaudeCodeCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, StrictModelName: true, ModelOverrideEnv: "CLAUDE_CODE_SUBAGENT_MODEL", InferSession: true, CanShowChildLine: true, SessionEnv: "CLAUDE_CODE_SESSION_ID", ChildEndEvent: "SubagentStop", UsageSource: UsageTranscript, HookEntries: []string{"settings-hook", "mod"}}

	// CursorCaps — preToolUse hook accepts updated_input.model.
	// Note: model rewrite is silently ignored on free/legacy plans.
	CursorCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, NativeSessionEnv: "DOWNSHIFT_CURSOR_NATIVE_FILE", NativeUsageSource: "cursor-usage", ChildEndEvent: "subagentStop", UsageSource: UsageNone}

	// CodexCaps — PreToolUse hook accepts updatedInput.model + reasoning_effort.
	// Child usage stays UsageNone until a real payload proves transcript_path is
	// the child's rollout and not the parent's (2026-10-13); the end status is
	// still recorded.
	CodexCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: true, TranscriptQuota: true, ChildEndEvent: "PostToolUse", UsageSource: UsageNone}

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

	// GrokCaps — one model with configurable reasoning in config.toml; there
	// is no hook, so it is deliberately not in AllHarnessCaps (doctor output
	// is unchanged).
	GrokCaps = HarnessCapabilities{SingleModelReasoning: true}
)

// AllHarnessCaps lists every declared harness in a stable order, so shared
// tooling (doctor, audits) can iterate harnesses without naming any of them.
func AllHarnessCaps() []NamedCaps {
	return []NamedCaps{
		{"claude-code", ClaudeCodeCaps},
		{"cursor", CursorCaps},
		{"codex", CodexCaps},
		{"kirocrew", KiroCrewCaps},
		{"antigravity", AntigravityCaps},
	}
}

func init() {
	// quota cannot import core, so the native export facts declared here are
	// handed to it once, at startup.
	for _, n := range AllHarnessCaps() {
		quota.RegisterNativeSource(n.Harness, n.Caps.NativeSessionEnv, n.Caps.NativeUsageSource)
	}
}

// CapsFor returns the declared capabilities of a harness by id.
func CapsFor(harness string) (HarnessCapabilities, bool) {
	for _, n := range AllHarnessCaps() {
		if n.Harness == harness {
			return n.Caps, true
		}
	}
	return HarnessCapabilities{}, false
}

// NamedCaps pairs a harness id with its declared capabilities.
type NamedCaps struct {
	Harness string
	Caps    HarnessCapabilities
}

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
