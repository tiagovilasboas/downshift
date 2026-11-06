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
	// ModelForceEnv names an environment variable that, when on, makes the
	// harness run every subagent on one model, ignoring both the per-call
	// model and a hook's rewrite (observed on Claude Code v2.1.310,
	// 2026-10-29). Empty means none is known.
	ModelForceEnv string
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
	// SessionFromCatalog means the harness accepts the catalog's
	// session_candidates names in every session and its payload carries no
	// session list, so an unknown session is built from those candidates
	// below the current model (see CatalogSessionFromCurrent). False keeps an
	// unknown session unknown: nothing is rewritten.
	SessionFromCatalog bool
	// CanShowChildLine means the hook protocol can show the developer one line
	// of text when a subagent ends (U2). Verified per harness; false until a
	// harness is shown to display it. The line is computed either way.
	CanShowChildLine bool
	// CanReturnNote means the PreToolUse answer can carry one short note the
	// orchestrator reads, with the spawn left unchanged (Decision.Advice). True
	// only where a probe showed the note reaches the model (Claude Code 2.1.316,
	// additionalContext). False: the note is computed and recorded as a reason
	// code only.
	CanReturnNote bool
	// SessionEnv names the environment variable that holds the harness's
	// current session id inside a command the developer runs from the harness
	// (U3, `downshift session`). Empty means the harness exports none, and the
	// command falls back to the most recent session in the event log.
	SessionEnv string
	// ShellEnv names an environment variable the harness sets in every shell
	// it spawns for the agent, used only to attribute compactor exec rows to
	// the harness (`envHarness`). Declared only when the harness's own source
	// or documentation shows it; empty means unverified, and the row stays
	// "unknown". Unlike SessionEnv it is never read as a session id.
	ShellEnv string
	// AgentEnv lists environment variables the harness sets in the shell it
	// spawns for the agent, whose presence means an agent (not a person) is
	// running the command. Used by the console surface decision to stay plain.
	// Declared only from the harness's own source or documentation; empty means
	// unverified, not absent.
	AgentEnv []string
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
	// RewriteImpliesApproval: the shell rewrite answer also approves the command
	// (Codex: updatedInput needs "allow"). UnknownUserRules: the user's deny/ask
	// rules cannot be read, so a rewrite could dodge one. Either one limits the
	// shared shell rewrite to one read-only command (execwrap.ReadOnly).
	RewriteImpliesApproval, UnknownUserRules bool
	// ShellDialect names the shell the harness runs its commands in. Only
	// ShellPOSIX lets the shared policy rewrite compound (`| tail -N`,
	// `cd d && cmd`) and chain (`a && b`, `a ; b`) shapes; empty means unknown.
	ShellDialect string
	// CanAdviseShellWrap means the hook can show the model one stderr line
	// (exit 2) suggesting a retry with the wrapped command. Used by harnesses
	// whose protocol is exit-0/exit-2 with no updated_input rewrite path.
	// KiroCrew uses this; set by A-P1.
	CanAdviseShellWrap bool
	// AgentTypeHints maps a subagent type name, exactly as the harness sends
	// it, to the tier that type's job needs (see RouteAgent). A type absent
	// from the map is no signal. Empty means no type has shown a stable
	// meaning on this harness; names are added only from evidence.
	AgentTypeHints map[string]Tier
	// SquadAgentPrefix is what precedes a squad agent's name in the type the
	// harness sends ("downshift:" for a Claude Code plugin agent). Empty means
	// the squad agent arrives by its bare name. The tier each agent declares is
	// read from agents/*.md by the shared decider (internal/routing/agenttier).
	SquadAgentPrefix string
	// MainToolEvent is the hook event that reports each tool call of the main
	// session, so Downshift can count mechanical work done without a subagent.
	// Empty means the harness delivers no such event: its row is "unobserved",
	// never zero. MainToolCategories maps the tool names the harness sends on
	// that event to a ToolCategory; a tool absent from it counts as "other".
	MainToolEvent      string
	MainToolCategories map[string]ToolCategory
	// MaxParallel is the most subagents the harness is known to run at once.
	// 0 means none is known (the policy or default cap applies). It is set only
	// from evidence, and ranks above the policy file, below the user's override.
	MaxParallel int
	// WorkerAgent names the worker agent (the CLI the queue launches) that runs
	// on this harness. Empty means the harness has no worker agent.
	WorkerAgent string
	// WorkerReadOnlyVerified means a read-only worker on this harness was
	// verified live to be unable to write (docs/evidence/mcp-claude-code-2026-10-31.md).
	WorkerReadOnlyVerified bool
}

// ShellPOSIX is the HarnessCapabilities.ShellDialect of a POSIX shell.
const ShellPOSIX = "posix"

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
	ClaudeCodeCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, RewritePreservesInput: true, StrictModelName: true, ModelOverrideEnv: "CLAUDE_CODE_SUBAGENT_MODEL", ModelForceEnv: "CLAUDE_CODE_SUBAGENT_MODEL_FORCE", SessionFromCatalog: true, CanShowChildLine: true, CanReturnNote: true, SessionEnv: "CLAUDE_CODE_SESSION_ID", ChildEndEvent: "SubagentStop", UsageSource: UsageTranscript, HookEntries: []string{"settings-hook", "mod"}, CanRewriteShellCommand: true, ShellDialect: ShellPOSIX, SquadAgentPrefix: "downshift:", WorkerAgent: "claude", WorkerReadOnlyVerified: true, MainToolEvent: "PostToolUse", MainToolCategories: claudeCodeToolCategories,
		// Explore is read-only search, Plan is design work (built-in types).
		AgentTypeHints: map[string]Tier{"Explore": TierSmall, "Plan": TierMid}}

	// CursorCaps — preToolUse hook accepts updated_input.model.
	// Model rewrite is silently ignored on free/legacy plans. Shell:
	// updated_input.command is documented, unverified live; the advise
	// fallback (beforeShellExecution agent_message) never denies.
	CursorCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, RewritePreservesInput: true, CanRewriteShellCommand: true, ShellDialect: ShellPOSIX, UnknownUserRules: true, CanAdviseShellWrap: true, NativeSessionEnv: "DOWNSHIFT_CURSOR_NATIVE_FILE", NativeUsageSource: "cursor-usage", ChildEndEvent: "subagentStop", UsageSource: UsageNone,
		// Cursor's common hook input; Claude Code sends none of these. A live
		// preToolUse on 3.13 carried conversation_id, generation_id and
		// workspace_roots (docs/evidence/vendor-hook-docs-2026-10-11.md).
		PayloadMarkers: []string{"cursor_version", "conversation_id", "generation_id", "workspace_roots"}}

	// CodexCaps — PreToolUse hook accepts updatedInput.model + reasoning_effort.
	// Child usage stays UsageNone until transcript_path is proven the child's
	// rollout, not the parent's (2026-10-13); the end status is still recorded.
	// ShellEnv: CODEX_THREAD_ID is injected into every shell command the agent
	// runs (openai/codex codex-rs/core/src/exec_env.rs `create_env`, constant
	// in codex-rs/protocol/src/shell_environment.rs; checked 2026-11-03).
	CodexCaps = HarnessCapabilities{ShellEnv: "CODEX_THREAD_ID", CanRewriteModel: true, CanApplyEffort: true, RewritePreservesInput: true, TranscriptQuota: true, ChildEndEvent: "PostToolUse", UsageSource: UsageNone, CanRewriteShellCommand: true, ShellDialect: ShellPOSIX, RewriteImpliesApproval: true, UnknownUserRules: true, WorkerAgent: "codex"}

	// KiroCrewCaps — KiroCrew's preToolUse hook is policy-only: its contract is
	// exit 0 (allow) / exit 2 (block + stderr to the LLM), with NO updated_input
	// rewrite path. So the model cannot be replaced in place; the adapter blocks
	// a mismatched spawn and instructs the agent to respawn at the right tier.
	// Child end (N5): postToolUse carries the spawn result; usage stays None.
	// CanAdviseShellWrap: a shell call of a compressing family is answered with
	// exit 2 and the wrapped command, so the model retries it compacted.
	// SessionEnv: KIRO_SESSION_ID is set by the KiroCrew gateway for every agent
	// process, so envHarness() can identify compactor calls from KiroCrew.
	KiroCrewCaps = HarnessCapabilities{CanRewriteModel: false, CanApplyEffort: false, ChildEndEvent: "postToolUse", UsageSource: UsageNone, CanAdviseShellWrap: true, SessionEnv: "KIRO_SESSION_ID"}

	// AntigravityCaps — the Antigravity hook accepts a model override on the
	// subagent payload. The wire format carries tier aliases (flash_lite,
	// flash, pro), not catalog ids, so the adapter maps the routed tier to
	// the alias and writes it only when the session plan allows a rewrite.
	// No effort field. Shell rewrite (run_command) is documented, not live-verified.
	// No ShellEnv: none is documented (same for Cursor).
	AntigravityCaps = HarnessCapabilities{CanRewriteModel: true, CanApplyEffort: false, RewritePreservesInput: true, CanRewriteShellCommand: true, ShellDialect: ShellPOSIX, UnknownUserRules: true}

	// OpenCodeCaps — the OpenCode plugin forwards subagent_spawn_intent to
	// `downshift opencode` and applies a {action:"route", model:{providerID,
	// id}} answer to the child before the spawn; CanRewriteModel is true
	// because the wire carries the target model rather than editing the spawn
	// input, so RewritePreservesInput is false. Effort has no wire field
	// (CanApplyEffort false). ChildEndEvent is the child_done observation
	// envelope the end hook parses. UsageSource is payload: the plugin sums
	// the per-step token report (session.step.ended) and sends it with the
	// child_done envelope, so a child's usage is recorded from the payload.
	// CanRewriteShellCommand: the plugin replaces the command of the shell
	// tool (execute.before) with the answer the adapter gives.
	// ShellEnv: the bash tool exports OPENCODE_SESSION_ID (and OPENCODE=1) to
	// the command (anomalyco/opencode packages/core/src/tool/plugin/shell.ts;
	// checked 2026-11-03).
	OpenCodeCaps = HarnessCapabilities{ShellEnv: "OPENCODE_SESSION_ID", AgentEnv: []string{"OPENCODE"}, CanRewriteModel: true, CanApplyEffort: false, ChildEndEvent: "child_done", UsageSource: UsagePayload, CanRewriteShellCommand: true, ShellDialect: ShellPOSIX}

	// GrokCaps — one model, configurable reasoning; no hook, so deliberately
	// not in AllHarnessCaps.
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
