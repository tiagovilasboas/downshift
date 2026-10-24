// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// HarnessCapabilities describes what a harness's hook protocol can carry.
// Routing policy is shared; adapters only declare what their wire format
// supports. Add a new harness by adding a named var below — no adapter code
// needs to change.
type HarnessCapabilities struct {
	CanRewriteModel bool // hook output can replace the subagent model ID
	CanApplyEffort  bool // hook output can set reasoning effort level
	// RewritePreservesInput means the rewrite channel echoes the spawn's own
	// input back (updatedInput-style), so every sibling field survives the
	// rewrite. False where the hook answers with a route decision the harness
	// applies itself, without echoing the spawn input.
	RewritePreservesInput bool
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
	// NativeUsageSource is the native usage parser source name for that export.
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
	// PayloadMarkers are top-level payload keys only this harness sends. A
	// harness that also runs another harness's hooks (Cursor runs the Claude
	// Code hooks in ~/.claude/settings.json when third-party configs are on)
	// delivers the same spawn twice; the copy carrying a foreign marker is left
	// to its own harness's hook (see ForeignPayloadOwner). Empty means none known.
	PayloadMarkers []string
	// CanRewriteShellCommand means the hook can replace the command of the
	// harness's shell tool in place (the opt-in `downshift exec` wrapper).
	// False means the wrap decision is still computed by the shared runner
	// when the adapter exposes the command, but never written.
	CanRewriteShellCommand bool
	// CanAdviseShellWrap means the hook can show the model one stderr line
	// (exit 2) suggesting a retry with the wrapped command. Used by harnesses
	// whose protocol is exit-0/exit-2 with no updated_input rewrite path.
	// KiroCrew uses this; set by A-P1.
	CanAdviseShellWrap bool
	// CanInjectPromptContext means the harness's prompt-submit hook can return
	// extra context for the model (the opt-in code-intelligence hook, run by
	// the shared prompt-context runner). False means the hook is never run.
	CanInjectPromptContext bool
	// AgentTypeHints maps a subagent type name, exactly as the harness sends
	// it, to the tier that type's job needs (see RouteAgent). A type absent
	// from the map is no signal. Empty means no type has shown a stable
	// meaning on this harness; names are added only from evidence.
	AgentTypeHints map[string]Tier
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
	// The same hook can return updatedInput.command for the Bash tool
	// (CanRewriteShellCommand, opt-in exec wrapper).
	ClaudeCodeCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, RewritePreservesInput: true, StrictModelName: true, ModelOverrideEnv: "CLAUDE_CODE_SUBAGENT_MODEL", InferSession: true, CanShowChildLine: true, SessionEnv: "CLAUDE_CODE_SESSION_ID", ChildEndEvent: "SubagentStop", UsageSource: UsageTranscript, HookEntries: []string{"settings-hook", "mod"}, CanRewriteShellCommand: true, CanInjectPromptContext: true,
		// Explore is read-only search, Plan is design work (built-in types).
		AgentTypeHints: map[string]Tier{"Explore": TierSmall, "Plan": TierMid}}

	// CursorCaps — preToolUse hook accepts updated_input.model.
	// Note: model rewrite is silently ignored on free/legacy plans.
	CursorCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, RewritePreservesInput: true, NativeSessionEnv: "DOWNSHIFT_CURSOR_NATIVE_FILE", NativeUsageSource: "cursor-usage", ChildEndEvent: "subagentStop", UsageSource: UsageNone, CanInjectPromptContext: true,
		// Cursor's common hook input; Claude Code sends none of these. A live
		// preToolUse on 3.13 carried conversation_id, generation_id and
		// workspace_roots (docs/evidence/vendor-hook-docs-2026-10-11.md).
		PayloadMarkers: []string{"cursor_version", "conversation_id", "generation_id", "workspace_roots"}}

	// CodexCaps — PreToolUse hook accepts updatedInput.model + reasoning_effort.
	// Child usage stays UsageNone until a real payload proves transcript_path is
	// the child's rollout and not the parent's (2026-10-13); the end status is
	// still recorded.
	CodexCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: true, RewritePreservesInput: true, TranscriptQuota: true, ChildEndEvent: "PostToolUse", UsageSource: UsageNone, CanRewriteShellCommand: true, CanInjectPromptContext: true}

	// KiroCrewCaps — KiroCrew's preToolUse hook is policy-only: its contract is
	// exit 0 (allow) / exit 2 (block + stderr to the LLM), with NO updated_input
	// rewrite path. So the model cannot be replaced in place; the adapter blocks
	// a mismatched spawn and instructs the agent to respawn at the right tier.
	// Child end (N5): postToolUse carries the spawn result; UsageSource stays
	// UsageNone until a real payload proves where the child's tokens come from.
	// CanAdviseShellWrap: a shell call of a compressing family is answered with
	// exit 2 and the wrapped command, so the model retries it compacted.
	KiroCrewCaps = HarnessCapabilities{CanRewriteModel: false, CanApplyEffort: false, ChildEndEvent: "postToolUse", UsageSource: UsageNone, CanAdviseShellWrap: true}

	// AntigravityCaps — the Antigravity hook accepts a model override on the
	// subagent payload. The wire format carries tier aliases (flash_lite,
	// flash, pro), not catalog ids, so the adapter maps the routed tier to
	// the alias and writes it only when the session plan allows a rewrite.
	// Effort is not supported: there is no reasoning-effort field.
	AntigravityCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, RewritePreservesInput: true}

	// OpenCodeCaps — the OpenCode plugin forwards subagent_spawn_intent to
	// `downshift opencode` and applies a {action:"route", model:{providerID,
	// id}} answer to the child before the spawn; CanRewriteModel is true
	// because the wire carries the target model rather than editing the spawn
	// input, so RewritePreservesInput is false. Effort has no wire field
	// (CanApplyEffort false). UsageSource stays UsageNone: no token seam is
	// proven yet (the bus fallback is NOT PROVEN — see
	// internal/adapters/opencode). ChildEndEvent is the child_done
	// observation envelope the end hook parses.
	OpenCodeCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, ChildEndEvent: "child_done", UsageSource: UsageNone, CanInjectPromptContext: true}

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
		{"opencode", OpenCodeCaps},
	}
}

// ForeignPayloadOwner returns the harness whose PayloadMarkers appear in a
// payload delivered to the hook of harness, or "" when the payload is that
// harness's own (or carries no marker). has reports whether a top-level key is
// present. The owner's own hook routes and records that spawn; the copy is a
// duplicate delivery and must not be routed or counted a second time.
func ForeignPayloadOwner(harness string, has func(key string) bool) string {
	for _, n := range AllHarnessCaps() {
		if n.Harness == harness {
			continue
		}
		for _, key := range n.Caps.PayloadMarkers {
			if has(key) {
				return n.Harness
			}
		}
	}
	return ""
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
