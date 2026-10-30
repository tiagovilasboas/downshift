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

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// Event is one routing decision persisted to the event log.
// Prompt text is intentionally excluded — only routing metadata is stored.
//
// Row discriminator: the kind of record is identified by the Outcome field
// (its "outcome" JSON key), never a "type" field. The native harness
// transcript format (e.g. Claude Code JSON lines with {"type":"assistant",...})
// is a distinct schema and is never the downshift events.jsonl schema.
type Event struct {
	CorrelationID    string  `json:"correlation_id"`             // opaque ID generated per hook invocation
	Timestamp        string  `json:"timestamp"`                  // RFC3339Nano
	Source           string  `json:"source"`                     // hook|cli
	Agent            string  `json:"agent"`                      // subagent, never task text
	Harness          string  `json:"harness"`                    // e.g. "claude-code"
	SessionID        string  `json:"session_id,omitempty"`       // Codex session identifier, when provided
	Complexity       string  `json:"complexity"`                 // TRIVIAL|SIMPLE|MEDIUM|COMPLEX
	FromModel        string  `json:"requested_model"`            // model supplied by the harness
	ToModel          string  `json:"final_model"`                // selected model after policy
	RequestedEffort  string  `json:"requested_reasoning_effort"` // level, "inherit" or "unobserved"
	FinalEffort      string  `json:"final_reasoning_effort"`     // policy recommendation
	Verdict          string  `json:"verdict"`                    // DOWNSHIFT|UPSHIFT|OK|UNKNOWN
	Tier             string  `json:"tier"`                       // SMALL|MID|FRONTIER
	PolicyVersion    string  `json:"policy_version"`
	BinaryVersion    string  `json:"binary_version"`
	Outcome          string  `json:"outcome"` // rewrite_emitted|allow|blocked|error|usage|baseline
	ErrorCode        string  `json:"error_code,omitempty"`
	EstimatedSavings float64 `json:"estimated_savings"` // normalised fraction 0–1
	QuotaStatus      string  `json:"quota_status,omitempty"`
	QuotaSource      string  `json:"quota_source,omitempty"`
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

	// PermissionMode is the session's permission mode name on decision records
	// (see PermissionModeLabel): "plan", "auto", "default" and so on, never a
	// prompt or a path. It lets the report count decisions per mode, such as
	// plan mode, where opusplan's model choice is not otherwise visible. Empty
	// on events that predate the field and on harnesses that do not report it.
	PermissionMode string `json:"permission_mode,omitempty"`

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

	// Optional naive-Bayes TRIVIAL downshift opinion, present only when NB
	// would have lowered a no-signal MEDIUM default to small. Applied is false
	// in shadow mode (the default).
	NBDownshift *NBUpshiftRecord `json:"nb_downshift,omitempty"`

	// E5Shadow is the multilingual E5 shadow observation (opt-in env).
	// Prompt-free: labels, similarity, and error codes only.
	E5Shadow *E5ShadowRecord `json:"e5_shadow,omitempty"`

	// E5Fuse is the opt-in multilingual fuse trace (DOWNSHIFT_E5_FUSE=1):
	// what the helper answered and the action taken. Prompt-free; absent
	// when the flag is off.
	E5Fuse *E5FuseRecord `json:"e5_fuse,omitempty"`

	// RoutingPolicy and ReasonCodes are the prompt-free decision trace:
	// the deterministic policy version and the sorted codes of the signals
	// and rules that shaped this decision (see reason_codes.go). Absent on
	// events written before the trace existed and on non-decision records.
	RoutingPolicy string   `json:"routing_policy,omitempty"`
	ReasonCodes   []string `json:"reason_codes,omitempty"`
	// UpshiftCostRatio is the catalog list-price ratio (routed over requested,
	// input plus output per 1M) recorded on an UPSHIFT whose two prices are both
	// known and whose target costs more. It lets stats charge the extra cost.
	// Absent on old logs and when either price is unknown (then neutral).
	UpshiftCostRatio float64 `json:"upshift_cost_ratio,omitempty"`
	// GatewayWould is the slug of the allowlisted OpenRouter row that would
	// serve this tier (shadow only; the routed model is unchanged), and
	// GatewayWouldTier that tier. Slug and tier only: no prompt, no task text.
	GatewayWould     string `json:"gateway_would,omitempty"`
	GatewayWouldTier string `json:"gateway_would_tier,omitempty"`
	// GatewayPrice is set on a usage record whose child ran on an OpenRouter
	// model (ToModel is its vendor/model slug): "snapshot" when the price came
	// from the OpenRouter list snapshot, "unknown" when none was found. See
	// gateway_price.go. Absent on every other record.
	GatewayPrice string `json:"gateway_price,omitempty"`
	// Optional Code Intelligence (DCI) record. Set on OutcomeDCI events.
	DCI *DCIRecord `json:"dci,omitempty"`

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

// DCIRecord captures prompt-time code intelligence evaluation metrics.
// Prompt text, raw source code, and snippets are strictly excluded.
type DCIRecord struct {
	IndexedFiles        int    `json:"indexed_files,omitempty"`
	CandidateCount      int    `json:"candidate_count,omitempty"`
	SelectedFiles       int    `json:"selected_files,omitempty"`
	SelectedSymbols     int    `json:"selected_symbols,omitempty"`
	InputBytes          int    `json:"input_bytes,omitempty"`
	OutputBytes         int    `json:"output_bytes,omitempty"`
	IndexDurationMs     int64  `json:"index_duration_ms,omitempty"`
	RetrievalDurationMs int64  `json:"retrieval_duration_ms,omitempty"`
	PackDurationMs      int64  `json:"pack_duration_ms,omitempty"`
	TotalDurationMs     int64  `json:"total_duration_ms,omitempty"`
	InjectionApplied    bool   `json:"injection_applied"`
	AbstentionReason    string `json:"abstention_reason,omitempty"`
	ErrorCode           string `json:"error_code,omitempty"`
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

// E5ShadowRecord is a parallel embedding observation; it never carries prompt text.
type E5ShadowRecord struct {
	ModelID        string  `json:"model_id,omitempty"`
	NearestLabel   string  `json:"nearest_label,omitempty"`
	Similarity     float64 `json:"similarity,omitempty"`
	WouldLabel     string  `json:"would_label,omitempty"`
	WouldLift      bool    `json:"would_lift,omitempty"`
	RegexLabel     string  `json:"regex_label,omitempty"`
	RegexConfident bool    `json:"regex_confident,omitempty"`
	LatencyMs      float64 `json:"latency_ms,omitempty"`
	ErrorCode      string  `json:"error_code,omitempty"`
}

// E5FuseRecord is the production fuse trace. Its fields are the closed set
// {model_id, nearest_label, similarity, action, latency_ms, error_code}; no
// prompt, vector, stderr or env path is ever added.
type E5FuseRecord struct {
	ModelID      string  `json:"model_id,omitempty"`
	NearestLabel string  `json:"nearest_label,omitempty"`
	Similarity   float64 `json:"similarity,omitempty"`
	Action       string  `json:"action,omitempty"`
	LatencyMs    float64 `json:"latency_ms,omitempty"`
	ErrorCode    string  `json:"error_code,omitempty"`
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

var correlationIDPattern = regexp.MustCompile(`^[a-f0-9]{16,64}$`)
var modelIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,128}$`)
var rawModelPattern = regexp.MustCompile(`^[A-Za-z0-9._:/-]{1,64}$`)
var effortPattern = regexp.MustCompile(`^(none|minimal|low|medium|high|xhigh|max|ultra)$`)

// Session IDs are untrusted hook input. Validate a deliberately small opaque
// identifier vocabulary before hashing so arbitrary user-controlled payloads
// never enter even a derived local record.
var sessionIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// NewCorrelationID returns an opaque, prompt-free ID that joins one hook
// invocation to its event-log record.
func NewCorrelationID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// CorrelationIDOrNew accepts only the opaque hexadecimal format. Invalid
// caller input is replaced rather than copied into local observability data.
func CorrelationIDOrNew(candidate string) string {
	if correlationIDPattern.MatchString(candidate) {
		return candidate
	}
	return NewCorrelationID()
}

// ModelOrUnknown bounds untrusted hook metadata before durable logging.
func ModelOrUnknown(candidate string) string {
	if modelIDPattern.MatchString(candidate) {
		return candidate
	}
	return "unknown"
}

// RawModelOrInvalid bounds an unrecognised requested model before logging.
// Anything that does not look like a short model identifier (for example
// text with spaces) becomes "invalid", so prompt text never reaches the log.
func RawModelOrInvalid(candidate string) string {
	if rawModelPattern.MatchString(candidate) {
		return candidate
	}
	return "invalid"
}

// clientPattern bounds the harness client label: a short, lower-case token.
// CLAUDE_CODE_ENTRYPOINT is untrusted, so anything with other characters or
// over the length cap is dropped rather than stored.
var clientPattern = regexp.MustCompile(`^[a-z0-9._-]{1,32}$`)

// ClientOrDrop returns candidate when it is a short lower-case token
// (at most 32 chars, [a-z0-9._-]), and "" otherwise so the field is dropped.
// It never trusts the value for policy; it only labels the event.
func ClientOrDrop(candidate string) string {
	if clientPattern.MatchString(candidate) {
		return candidate
	}
	return ""
}

// Reasons recorded in place of a reasoning effort level.
const (
	// EffortInherit is a spawn that sets no effort of its own, so the harness
	// applies its default (Claude Code: the session effort).
	EffortInherit = "inherit"
	// EffortUnobserved is a hook payload that carries no effort at all.
	EffortUnobserved = "unobserved"
)

// EffortOrUnknown accepts only the cross-harness effort vocabulary and
// EffortInherit. An empty value is EffortUnobserved; anything else is
// "unknown", so a malformed value never reaches the log.
func EffortOrUnknown(candidate string) string {
	switch {
	case effortPattern.MatchString(candidate), candidate == EffortInherit:
		return candidate
	case candidate == "":
		return EffortUnobserved
	}
	return "unknown"
}

// HashSessionID avoids persisting a harness-provided session identifier.
func HashSessionID(sessionID string) string {
	if !sessionIDPattern.MatchString(sessionID) {
		return ""
	}
	sum := sha256.Sum256([]byte("harness-downshift-session-v1:" + sessionID))
	return hex.EncodeToString(sum[:])
}

// Outcome values for hook routing events.
const (
	// OutcomeRewriteEmitted marks a hook response that changed the spawn's model.
	OutcomeRewriteEmitted = "rewrite_emitted"
	// OutcomeAllow marks a routing decision whose hook response left the spawn unchanged.
	OutcomeAllow = "allow"
	// OutcomeBlocked marks a policy-mode hook (KiroCrew) that denied the
	// spawn and asked the agent to respawn on another model. Nothing was
	// rewritten.
	OutcomeBlocked = "blocked"
	// OutcomeFailOpen marks a hook that could not answer (malformed input,
	// internal error, oversized payload): the spawn was allowed unchanged and
	// the hook's own fail-open response was emitted. It records no decision.
	OutcomeFailOpen = "fail_open"
)

// IsFailureOutcome reports whether an outcome is a hook failure rather than a
// routing decision: an error record or a fail-open answer. Failure records
// count in no routed/decision aggregate, exactly like errors.
func IsFailureOutcome(outcome string) bool {
	return outcome == "error" || outcome == OutcomeFailOpen
}

// AppliedRewrite reports whether a decision event changed the spawn: the
// hook emitted a rewrite and no guardrail held the decision. Allow events,
// held decisions (corrections present) and blocks never count as savings.
// Lines written before the outcome field existed (empty outcome) count as
// emitted for backward compatibility.
func AppliedRewrite(outcome string, corrections []string) bool {
	if len(corrections) > 0 {
		return false
	}
	return outcome == OutcomeRewriteEmitted || outcome == ""
}

// AppliedRouting reports whether a decision event effectively changed the
// model that will run. This is broader than AppliedRewrite: it also counts
// blocked outcomes where the harness is forced to respawn on a different
// model (e.g. KiroCrew's allow/block-only contract).
func AppliedRouting(ev Event) bool {
	if len(ev.Corrections) > 0 {
		return false
	}
	// Classic rewrite: hook modified the spawn in place
	if ev.Outcome == OutcomeRewriteEmitted || ev.Outcome == "" {
		return true
	}
	// Blocked with model change: harness must respawn on the recommended model
	if ev.Outcome == OutcomeBlocked && ev.ToModel != "" && ev.FromModel != "" && ev.ToModel != ev.FromModel {
		return true
	}
	return false
}

// MarkUnchanged rewrites final_model/final_reasoning_effort on an event
// whose hook response left the spawn unchanged (allow or baseline), so the
// log records what the child actually runs on. The recommendation moves to
// recommended_model. Rewrite and blocked events are left as they are.
func MarkUnchanged(ev *Event) {
	if ev.Outcome != OutcomeAllow && ev.Outcome != OutcomeBaseline {
		return
	}
	if ev.ToModel != ev.FromModel {
		ev.RecommendedModel = ev.ToModel
	}
	ev.ToModel = ev.FromModel
	ev.FinalEffort = ev.RequestedEffort
}

// Failure records a hook fail-open path without accepting raw input or errors
// that could contain prompt content.
func Failure(correlationID, binaryVersion, harness, errorCode string) Event {
	return Event{
		CorrelationID: correlationID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		Harness:       harness,
		PolicyVersion: PolicyVersion,
		BinaryVersion: binaryVersion,
		Outcome:       "error",
		ErrorCode:     errorCode,
	}
}

// FailOpen records one harness answer that could not route a spawn: the hook
// emitted its fail-open response and the child ran unchanged. outcome is
// "fail_open" (never "error") so fail-open rates stay countable apart from
// hard failures; cause is a short code such as the wire's "INVALID_REQUEST",
// never raw input.
func FailOpen(correlationID, binaryVersion, harness, cause string) Event {
	return Event{
		CorrelationID: correlationID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		Harness:       harness,
		PolicyVersion: PolicyVersion,
		BinaryVersion: binaryVersion,
		Outcome:       OutcomeFailOpen,
		ErrorCode:     cause,
	}
}

// defaultEventPath returns the routing telemetry JSONL path under the state dir.
func defaultEventPath() string {
	if path := os.Getenv("DOWNSHIFT_EVENT_LOG"); path != "" {
		return path
	}
	p, err := paths.EventsPath()
	if err != nil {
		return "events.jsonl"
	}
	return p
}

var mu sync.Mutex

// lockTimeout bounds how long a hook waits for the event-log lock. It stays
// well below every harness hook budget; a lost event is reported, never
// allowed to stall a spawn.
const lockTimeout = 500 * time.Millisecond

// recordErrOut receives the one-line warning when an event cannot be
// written. Tests replace it.
var recordErrOut io.Writer = os.Stderr

// Record appends ev to the default event log. A telemetry failure never
// interrupts a hook, but it is not silent either: one short line goes to
// stderr so a lost event is visible.
func Record(ev Event) {
	canonicalizeModels(&ev)
	if err := AppendTo(defaultEventPath(), ev); err != nil {
		fmt.Fprintf(recordErrOut, "downshift: telemetry event not recorded: %v\n", err)
	}
}

// AppendTo appends ev to the given path, creating the file and parent
// directories as needed. Returns any write error.
//
// Concurrency note: the in-process mutex (mu) prevents data races between
// goroutines in the same binary. Multi-process safety comes from LockFile:
// flock(2) on Unix (released by the kernel when a holder dies, so a killed
// hook cannot orphan it) and a PID lock file with stale detection elsewhere.
// On POSIX systems, once a process owns the lock, O_APPEND writes are
// atomic at the kernel level for sizes < PIPE_BUF (~4 KB), so JSON lines
// will not interleave even under contention.
func AppendTo(path string, ev Event) error {
	mu.Lock()
	defer mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// Acquire exclusive lock (multi-process safe).
	lock, err := LockFile(path, lockTimeout)
	if err != nil {
		return fmt.Errorf("failed to acquire lock on %s: %w", path, err)
	}
	defer lock.Close()

	if _, err := RotationPolicyFromEnv().MaybeRotate(path); err != nil {
		var cleanup *BackupCleanupError
		if !errors.As(err, &cleanup) {
			return fmt.Errorf("log rotation failed for %s: %w", path, err)
		}
		// Rotation worked; stale backups remain. Say so, keep recording.
		fmt.Fprintf(os.Stderr, "downshift: %v\n", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	// Tighten permissions on existing logs too; OpenFile's mode only applies
	// when creating a file, and event metadata can reveal model usage patterns.
	if err := f.Chmod(0o600); err != nil {
		return err
	}

	line, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "%s\n", line); err != nil {
		return err
	}
	// No fsync: the write(2) is in the OS buffer cache and survives a process
	// crash. On macOS Go's File.Sync is F_FULLFSYNC (~2.5 ms measured), paid on
	// every hook spawn for an advisory log. A power loss can drop the last
	// event; the log is statistics, not a ledger.
	return nil
}

// Stats is the aggregated view of all recorded events.
type Stats struct {
	Total       int
	Downshifted int
	Upshifted   int
	OK          int
	Unknown     int

	// NormBaseline and NormRouted are normalised cost units (1 unit = 1 baseline event).
	// They do not represent real dollars. Multiply by CostPerUnit to convert.
	NormBaseline float64
	NormRouted   float64

	// UsageRecords counts all post-hoc usage records, including unmatched ones.
	// UsageLinked counts distinct usage records whose references resolve to
	// decisions in the same harness and compatible session. Records are deduped
	// by agent hash when supplied, otherwise their own event identity.
	// Baseline counts control-group events recorded with routing disabled
	// (outcome "baseline", see DOWNSHIFT_NO_ROUTE): classified but never
	// applied, excluded from decision counts so before/after comparison
	// stays honest.
	UsageRecords int
	UsageLinked  int
	Baseline     int

	// PolicyFloor counts decisions an operator tier floor raised.
	PolicyFloor int
	// ByEntry counts decisions by spawn-time entry (hook, mod). Empty on
	// events written before the field existed are counted under EntryHook.
	ByEntry map[string]int
	// DuplicateToolUse counts distinct tool_use_hash values that appear on
	// more than one decision (both entry points live on the same spawn).
	DuplicateToolUse int
	// GatewayObserved counts decisions an allowlisted OpenRouter row would
	// have served; GatewaySlugs counts them per slug. Shadow only.
	GatewayObserved int
	GatewaySlugs    map[string]int
	// ChildStatus counts how children ended (child_ended events), by status.
	ChildStatus map[ChildStatus]int
	// RespawnAfterFailure counts failed children followed by another spawn in
	// the same session within respawnWindow. A hint, not a verdict.
	RespawnAfterFailure int

	// ReasonCodes counts decisions per reason code. Events written before
	// the decision trace carry no codes and add nothing here.
	ReasonCodes map[string]int
	// FrontierDecisions counts decisions that picked the frontier tier.
	// FrontierOnDoubt counts those that rest on an unconfident classification
	// with no deterministic lift (risk floor, graph escalation, NB upshift, tier
	// floor): the silent-overspend shape. It should stay at zero.
	FrontierDecisions int
	FrontierOnDoubt   int

	// Corrected counts events where a guardrail held the decision: the
	// classified verdict stands in the log, but adapters applied the safe
	// action (see Event.Corrections for the rule IDs).
	Corrected int

	// NotApplied counts DOWNSHIFT/UPSHIFT decisions the hook did not apply
	// (outcome allow, blocked, or held by a guardrail). They are excluded
	// from Downshifted/Upshifted and from normalised savings.
	NotApplied int

	// ByComplexity counts decisions per complexity class.
	ByComplexity map[string]int
	// ByAgentType counts decisions per subagent type, and how many of them had no
	// known baseline model. Events without a type are not counted here.
	ByAgentType map[string]AgentTypeCount
	// ModelMix counts decisions per model the child ended up on (final_model,
	// short family name). Decisions with no recorded model add nothing.
	ModelMix map[string]int

	// RewriteShifted counts rewrite_emitted events that asked for a different model.
	// RewriteHonored is the inferred count from CountInferredHonored. The
	// report does not print it as honor. HonorObserved counts applied rewrites with
	// a linked resolved record whose rewrite_honored is true; HonorUnobserved the
	// applied rewrites with none. Spawns left unchanged are in neither.
	RewriteShifted  int
	RewriteHonored  int
	HonorObserved   int
	HonorUnobserved int
	UsageObserved   int
	UsageUnobserved int
	QuotaAvailable  int
	QuotaHeld       int
	QuotaUnknown    int

	// ObservedChildTokens is the sum of input, output, and cached tokens from
	// usage events that carry token counts. ObservedChildUsage is how many
	// such usage events contributed. Zero usage means unobserved, not measured zero.
	ObservedChildTokens int64
	ObservedChildUsage  int

	// TokensRerouted is the token view of routing, per harness: the tokens that
	// ran on a cheaper or a pricier model than the one requested (see
	// tokens_rerouted.go). Counts only; not tokens saved.
	TokensRerouted TokensRerouted

	// DCI holds aggregated Code Intelligence telemetry for the time window.
	DCI DCIStats
}

// DCIStats contains aggregated telemetry for Downshift Code Intelligence (DCI).
// In accordance with Downshift's epistemic invariants, all metrics are strictly
// observed; no synthetic token counts or fictitious dollar savings are computed.
type DCIStats struct {
	PromptsEvaluated    int            `json:"prompts_evaluated"`
	InjectionsApplied   int            `json:"injections_applied"`
	AbstentionsByReason map[string]int `json:"abstentions_by_reason,omitempty"`
	ContextBytesEmitted int64          `json:"context_bytes_emitted"`
	AvgContextBytes     float64        `json:"avg_context_bytes,omitempty"`
	LatencyP50Ms        float64        `json:"latency_p50_ms,omitempty"`
	LatencyP95Ms        float64        `json:"latency_p95_ms,omitempty"`
	Errors              int            `json:"errors,omitempty"`
	ErrorsByCode        map[string]int `json:"errors_by_code,omitempty"`
}

// NormSaved returns normalised savings: absolute units saved and fraction.
func (s Stats) NormSaved() (float64, float64) {
	saved := s.NormBaseline - s.NormRouted
	if s.NormBaseline <= 0 {
		return 0, 0
	}
	return saved, saved / s.NormBaseline
}

// ReadEvents reads all events from the default event log. Returns an empty
// slice (not an error) when the file does not exist.
func ReadEvents() ([]Event, error) {
	return ReadEventsFrom(defaultEventPath())
}

// ReadEventsFrom reads all events from path.
func ReadEventsFrom(path string) ([]Event, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseEvents(f)
}

func parseEvents(r io.Reader) ([]Event, error) {
	var events []Event
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), MaxLineBytes)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev Event
		if err := json.Unmarshal(line, &ev); err != nil {
			continue // skip malformed lines — never block on corrupted log
		}
		events = append(events, ev)
	}
	linkLateUsage(events)
	return events, sc.Err()
}

// FilterByDays returns events within the last n days.
// When n <= 0, all events are returned.
func FilterByDays(events []Event, days int) []Event {
	if days <= 0 {
		return events
	}
	cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
	var out []Event
	for _, ev := range events {
		// Events are written with time.RFC3339Nano (FromDecision, Failure);
		// time.RFC3339 cannot parse fractional seconds, so try the nano
		// layout first and accept the legacy layout for older log lines.
		// A timestamp that parses in neither layout is excluded
		// (fail-closed for windowing: a dateless event belongs to no
		// window, and silently including it would inflate every window).
		t, err := time.Parse(time.RFC3339Nano, ev.Timestamp)
		if err != nil {
			t, err = time.Parse(time.RFC3339, ev.Timestamp)
		}
		if err != nil {
			continue
		}
		if t.After(cutoff) {
			out = append(out, ev)
		}
	}
	return out
}

// respawnWindow is how soon after a failed child a new spawn in the same
// session counts as a respawn.
const respawnWindow = 10 * time.Minute

// tallySpawnSurface counts observed honor and usage from linked port records,
// and quota from each decision's own status. It does not treat a later spawn
// or rewrite_emitted as honor.
func tallySpawnSurface(events []Event, s *Stats) {
	honor, usage := ObservedLinks(events)
	for _, ev := range events {
		if ev.Harness == "" || IsCostOnlyOutcome(ev.Outcome) || IsFailureOutcome(ev.Outcome) {
			continue
		}
		surface := SurfaceFor(ev, honor[SurfaceKey(ev)], usage[SurfaceKey(ev)])
		// Honor is asked of a rewrite only: a spawn the hook left alone has no
		// written model to confirm, so counting it "unobserved" buried the few
		// real rewrites under every unchanged spawn.
		if isShift(ev) {
			switch surface.Honor {
			case HonorObserved:
				s.HonorObserved++
			default:
				s.HonorUnobserved++
			}
		}
		switch surface.Usage {
		case UsageObserved:
			s.UsageObserved++
		default:
			s.UsageUnobserved++
		}
		switch surface.Quota {
		case QuotaAvailable:
			s.QuotaAvailable++
		case QuotaHeld:
			s.QuotaHeld++
		default:
			s.QuotaUnknown++
		}
	}
}

// spentOnDoubt reports whether a frontier decision actually wrote the model on
// an unconfident classification with no deterministic lift. A decision a
// guardrail held, or one that left the spawn unchanged, spent nothing and is not
// counted: the alarm must only fire for money that was really spent.
func spentOnDoubt(ev Event) bool {
	return strings.EqualFold(ev.Tier, TierFrontier) &&
		ev.Outcome == OutcomeRewriteEmitted && AppliedRewrite(ev.Outcome, ev.Corrections) &&
		frontierOnDoubt(ev.ReasonCodes)
}

// frontierOnDoubt reports whether a decision's reason codes show an unconfident
// classification that no deterministic rule lifted.
func frontierOnDoubt(codes []string) bool {
	unconfident := false
	for _, c := range codes {
		switch c {
		case ReasonClassifierUnconfident:
			unconfident = true
		case ReasonRiskFloor, ReasonNBUpshiftApplied, ReasonTierFloor:
			return false
		}
	}
	return unconfident
}
