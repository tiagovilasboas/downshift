// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package telemetry records routing decisions to a local JSONL file and
// renders aggregated stats. All data stays on the user's machine — nothing
// is sent to any external service.
//
// Privacy note: prompt contents are never stored. Each event contains only
// routing metadata (harness, complexity class, model IDs, verdict, and a
// normalised savings fraction). This makes the event log safe for corporate
// environments where task prompts may contain sensitive information.
//
// Event log location: $DOWNSHIFT_STATE_DIR/events.jsonl or ~/.downshift/ (legacy: ~/.harness-downshift/).
// Each line is one JSON object written atomically (append + sync).
package telemetry

// Event is one routing decision persisted to the event log.
// Prompt text is intentionally excluded — only routing metadata is stored.
//
// Row discriminator: the kind of record is identified by the Outcome field
// (its "outcome" JSON key), never a "type" field. The native harness
// transcript format (e.g. Claude Code JSON lines with {"type":"assistant",...})
// is a distinct schema and is never the downshift events.jsonl schema.
type Event struct {
	CorrelationID   string `json:"correlation_id"`             // opaque ID generated per hook invocation
	Timestamp       string `json:"timestamp"`                  // RFC3339Nano
	Source          string `json:"source"`                     // hook|cli
	Agent           string `json:"agent"`                      // subagent, never task text
	Harness         string `json:"harness"`                    // e.g. "claude-code"
	SessionID       string `json:"session_id,omitempty"`       // Codex session identifier, when provided
	Complexity      string `json:"complexity"`                 // TRIVIAL|SIMPLE|MEDIUM|COMPLEX
	FromModel       string `json:"requested_model"`            // model supplied by the harness
	ToModel         string `json:"final_model"`                // selected model after policy
	RequestedEffort string `json:"requested_reasoning_effort"` // level, "inherit" or "unobserved"
	FinalEffort     string `json:"final_reasoning_effort"`     // policy recommendation
	Verdict         string `json:"verdict"`                    // DOWNSHIFT|UPSHIFT|OK|UNKNOWN
	Tier            string `json:"tier"`                       // SMALL|MID|FRONTIER
	PolicyVersion   string `json:"policy_version"`
	BinaryVersion   string `json:"binary_version"`
	Outcome         string `json:"outcome"` // rewrite_emitted|allow|blocked|error|usage|baseline
	ErrorCode       string `json:"error_code,omitempty"`
	// InvalidField names the payload field that failed validation (its wire
	// name only, never its value). Set on failure and fail-open rows.
	InvalidField string `json:"invalid_field,omitempty"`
	QuotaStatus  string `json:"quota_status,omitempty"`
	QuotaSource  string `json:"quota_source,omitempty"`
	// CreditHeld records that the credit gate refused this rewrite. It is
	// not a guardrail correction. Exhausted and stale status are holds
	// without this flag.
	CreditHeld bool `json:"credit_held,omitempty"`
	// PolicyFloor records that an operator tier floor raised this decision
	// above the classifier tier. Absent on old logs and when no floor applied.
	PolicyFloor bool `json:"policy_floor,omitempty"`
	// RequestedModelRaw keeps a non-empty requested model the catalog did not
	// recognise (requested_model is then "unknown"), so "auto" or an unlisted
	// id is distinguishable from an absent field. It also keeps the alias the
	// harness sent when requested_model was recorded as its catalog id (see
	// canonicalizeModels). Bounded by RawModelOrInvalid; absent when the
	// harness sent no model or sent the catalog id itself.
	RequestedModelRaw string `json:"requested_model_raw,omitempty"`
	// FinalModelRaw keeps the alias final_model had before it was recorded as
	// its catalog id. Absent when final_model was already the catalog id.
	FinalModelRaw string `json:"final_model_raw,omitempty"`

	// Optional real-cost fields (nil = provider usage not tracked for this event).
	// When absent, fall back to the normalised estimate above; never fake dollars.
	InputTokens      *int64 `json:"input_tokens,omitempty"`       // provider-reported input tokens
	OutputTokens     *int64 `json:"output_tokens,omitempty"`      // provider-reported output tokens
	CachedTokens     *int64 `json:"cached_tokens,omitempty"`      // provider-reported cached input tokens
	CacheWriteTokens *int64 `json:"cache_write_tokens,omitempty"` // cache-creation part of cached_tokens, billed at the cache-write price
	// BaselineSource says where the baseline model came from: "requested" (the
	// model the harness asked for) or "inferred" (the configured model, used
	// when the harness named none). Absent when no baseline was computed.
	BaselineSource string `json:"baseline_source,omitempty"`
	// ChildStatus is set on child_ended events (see child_outcome.go).
	ChildStatus ChildStatus `json:"child_status,omitempty"`
	// Acceptance records developer acceptance in git commit: "used" or "discarded".
	Acceptance string `json:"acceptance,omitempty"`
	// WorkerOutcome is how a delegated worker ended, on worker_ended records:
	// done, failed, timeout or cancelled (see worker_ended.go).
	WorkerOutcome string `json:"worker_outcome,omitempty"`
	// Tool carries the int-only counts of one MCP plan or delegate call, on
	// mcp_tool records (see mcp_tool.go).
	Tool *ToolCounts `json:"tool,omitempty"`

	// Optional self-correction record. Verdict keeps the classified value;
	// SafeVerdict carries the action adapters were allowed to apply, and
	// Corrections lists the guardrail rule IDs that held the decision.
	// Absent on clean decisions and on events written before self-check.
	SafeVerdict string   `json:"safe_verdict,omitempty"`
	Corrections []string `json:"corrections,omitempty"`

	// Optional post-spawn verification. Non-nil only when the harness or
	// an observer exposes the actual post-spawn child model or confirmation.
	// Privacy note: boolean only, no prompt or execution payloads stored.
	RewriteHonored *bool `json:"rewrite_honored,omitempty"`

	// LinkedDecision is set on "usage" records: the correlation id of the
	// decision event the usage was priced against. A decision is priced at
	// most once; later usage records skip it.
	LinkedDecision string `json:"linked_decision,omitempty"`

	// Verification is the subagent's own check result on "usage" records:
	// "passed" or "failed" when its transcript ran a recognised test or lint
	// command, "none" when it ran none. Absent on events written before this
	// field. Enum only: the command and its output are never stored.
	Verification string `json:"verification,omitempty"`

	// AgentHash is the hashed harness agent id on "resolved" and "usage"
	// records. It joins the spawn-time observation (PostToolUse) to the
	// completion-time one (SubagentStop) without storing the raw id.
	AgentHash string `json:"agent_hash,omitempty"`

	// AgentType is the subagent type of a decision, as a label: a built-in
	// name, "unspecified" or "custom" (see AgentTypeLabel). It lets stats say
	// which kinds of spawn had no known baseline. Empty on events that predate
	// the field and on harnesses that do not report a type.
	AgentType string `json:"agent_type,omitempty"`
	// AgentName is the squad agent's declared name ("coder") on a decision for
	// one of Downshift's own agents, whatever its harness spells the type. An id
	// from a fixed public list, never task text; empty for every other spawn.
	AgentName string `json:"agent_name,omitempty"`

	// PermissionMode is the session's permission mode name on decision records
	// (see PermissionModeLabel): "plan", "auto", "default" and so on, never a
	// prompt or a path. It lets the report count decisions per mode, such as
	// plan mode, where opusplan's model choice is not otherwise visible. Empty
	// on events that predate the field and on harnesses that do not report it.
	PermissionMode string `json:"permission_mode,omitempty"`

	// ParentModel is the model the parent session ran on when it spawned, and
	// ParentModelSource says how it was learned (see ParentModelLabel). Data
	// only: routing never reads them. Absent when the harness does not expose
	// it. With the orchestrator naming the child model, requested_model alone
	// cannot say whether the child ran below its parent; this can.
	ParentModel       string `json:"parent_model,omitempty"`
	ParentModelSource string `json:"parent_model_source,omitempty"`

	// ToolUseHash is the hashed harness tool-call id on decision records. The
	// post-spawn observations carry the same id, so they join the exact
	// decision instead of the latest sibling in the session.
	ToolUseHash string `json:"tool_use_hash,omitempty"`

	// Entry names which spawn-time path recorded this decision: EntryHook
	// (settings or plugin PreToolUse) or EntryMod (in-process agent.spawn).
	// Absent on events written before the field existed.
	Entry string `json:"entry,omitempty"`

	// RecommendedModel is the classifier's recommendation on events whose
	// spawn was left unchanged (allow, baseline, blocked). final_model on
	// those events is the model the child actually runs on.
	RecommendedModel string `json:"recommended_model,omitempty"`

	// Optional naive-Bayes second opinion, present only when NB would have
	// raised the tier. Applied is false in shadow mode (the default), where
	// the routing decision above is unchanged.
	NBUpshift *NBUpshiftRecord `json:"nb_upshift,omitempty"`

	// Optional naive-Bayes TRIVIAL downshift opinion. It is no longer emitted:
	// the opinion was removed. The field stays so old events that recorded one
	// still decode.
	NBDownshift *NBUpshiftRecord `json:"nb_downshift,omitempty"`

	// RoutingPolicy and ReasonCodes are the prompt-free decision trace:
	// the deterministic policy version and the sorted codes of the signals
	// and rules that shaped this decision (see reason_codes.go). Absent on
	// events written before the trace existed and on non-decision records.
	RoutingPolicy string   `json:"routing_policy,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`
	// GatewayWould is the slug of the allowlisted OpenRouter row that would
	// serve this tier (shadow only; the routed model is unchanged), and
	// GatewayWouldTier that tier. Slug and tier only: no prompt, no task text.
	GatewayWould     string `json:"gateway_would,omitempty"`
	GatewayWouldTier string `json:"gateway_would_tier,omitempty"`
	// RoutingNote is why this decision changed the spawn or kept it (see
	// RoutingNote). Absent on events written before the note was recorded.
	RoutingNote *RoutingNote `json:"routing_note,omitempty"`

	// Confidence is reserved and never set: the router has no calibrated
	// probability, so the key stays absent rather than carry a made-up number.
	Confidence *float64 `json:"confidence,omitempty"`

	// Client names the harness client that fired the hook, when the harness
	// exposes it: for Claude Code, CLAUDE_CODE_ENTRYPOINT (values seen in the
	// wild include "cli" and "claude-desktop"). It tells a Claude Desktop event
	// apart from a CLI one. The value is opaque and untrusted: sanitized by
	// ClientOrDrop and never used for policy. Absent on events written before
	// the field and on harnesses that do not report it.
	Client string `json:"client,omitempty"`
}

// NBUpshiftRecord is a tier change a naive-Bayes opinion would make (or made,
// when its flag is on): the upshift second opinion or the TRIVIAL downshift.
// Tiers only; no prompt text.
type NBUpshiftRecord struct {
	FromTier string  `json:"from_tier"`
	ToTier   string  `json:"to_tier"`
	Margin   float64 `json:"margin"`
	Applied  bool    `json:"applied"`
}

// HasTokenUsage reports whether the event carries any provider token count.
func (e Event) HasTokenUsage() bool { return classesOf(e.TokenUsageOrZero()).Total > 0 }

// TokenUsageOrZero returns the event token usage, nil-safe.
// Missing pointers and negative values are clamped to 0.
func (e Event) TokenUsageOrZero() TokenUsage {
	var u TokenUsage
	if e.InputTokens != nil && *e.InputTokens > 0 {
		u.InputTokens = *e.InputTokens
	}
	if e.OutputTokens != nil && *e.OutputTokens > 0 {
		u.OutputTokens = *e.OutputTokens
	}
	if e.CachedTokens != nil && *e.CachedTokens > 0 {
		u.CachedTokens = *e.CachedTokens
	}
	if e.CacheWriteTokens != nil && *e.CacheWriteTokens > 0 {
		u.CacheWriteTokens = *e.CacheWriteTokens
	}
	return u
}

const PolicyVersion = "core-route-v1"
