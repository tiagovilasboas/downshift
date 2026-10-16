// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// Event is one routing decision persisted to the event log.
// Prompt text is intentionally excluded — only routing metadata is stored.
type Event struct {
	CorrelationID        string                `json:"correlation_id"`             // opaque ID generated per hook invocation
	Timestamp            string                `json:"timestamp"`                  // RFC3339Nano
	Source               string                `json:"source"`                     // hook|cli
	Agent                string                `json:"agent"`                      // subagent, never task text
	Harness              string                `json:"harness"`                    // e.g. "claude-code"
	SessionID            string                `json:"session_id,omitempty"`       // Codex session identifier, when provided
	Complexity           string                `json:"complexity"`                 // TRIVIAL|SIMPLE|MEDIUM|COMPLEX
	FromModel            string                `json:"requested_model"`            // model supplied by the harness
	ToModel              string                `json:"final_model"`                // selected model after policy
	RequestedEffort      string                `json:"requested_reasoning_effort"` // absent from most hook protocols
	FinalEffort          string                `json:"final_reasoning_effort"`     // policy recommendation
	Verdict              string                `json:"verdict"`                    // DOWNSHIFT|UPSHIFT|OK|UNKNOWN
	Tier                 string                `json:"tier"`                       // SMALL|MID|FRONTIER
	PolicyVersion        string                `json:"policy_version"`
	BinaryVersion        string                `json:"binary_version"`
	Outcome              string                `json:"outcome"` // rewrite_emitted|allow|blocked|error|usage|baseline
	ErrorCode            string                `json:"error_code,omitempty"`
	DecisionIntelligence *ShadowRecommendation `json:"decision_intelligence,omitempty"`
	EstimatedSavings     float64               `json:"estimated_savings"` // normalised fraction 0–1
	QuotaStatus          string                `json:"quota_status,omitempty"`
	QuotaSource          string                `json:"quota_source,omitempty"`
	// CreditHeld records that the credit gate refused this rewrite. It is
	// not a guardrail correction. Exhausted and stale status are holds
	// without this flag.
	CreditHeld bool `json:"credit_held,omitempty"`
	// PolicyFloor records that an operator tier floor raised this decision
	// above the classifier tier. Absent on old logs and when no floor applied.
	PolicyFloor bool `json:"policy_floor,omitempty"`
	// RequestedModelRaw keeps a non-empty requested model the catalog did not
	// recognise (requested_model is then "unknown"), so "auto" or an unlisted
	// id is distinguishable from an absent field. Bounded by RawModelOrInvalid;
	// absent when the harness sent no model or the catalog recognised it.
	RequestedModelRaw string `json:"requested_model_raw,omitempty"`

	// Optional real-cost fields (nil = provider usage not tracked for this event).
	// When absent, fall back to the normalised estimate above; never fake dollars.
	InputTokens      *int64   `json:"input_tokens,omitempty"`       // provider-reported input tokens
	OutputTokens     *int64   `json:"output_tokens,omitempty"`      // provider-reported output tokens
	CachedTokens     *int64   `json:"cached_tokens,omitempty"`      // provider-reported cached input tokens
	CacheWriteTokens *int64   `json:"cache_write_tokens,omitempty"` // cache-creation part of cached_tokens, billed at the cache-write price
	ActualCostUSD    *float64 `json:"actual_cost_usd,omitempty"`    // real routed cost in USD
	BaselineCostUSD  *float64 `json:"baseline_cost_usd,omitempty"`  // real baseline cost in USD
	// BaselineSource says where the baseline model came from: "requested" (the
	// model the harness asked for) or "inferred" (the configured model, used
	// when the harness named none). Absent when no baseline was computed.
	BaselineSource string `json:"baseline_source,omitempty"`
	// ChildStatus is set on child_ended events (see child_outcome.go).
	ChildStatus ChildStatus `json:"child_status,omitempty"`

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

	// ToolUseHash is the hashed harness tool-call id on decision records. The
	// post-spawn observations carry the same id, so they join the exact
	// decision instead of the latest sibling in the session.
	ToolUseHash string `json:"tool_use_hash,omitempty"`

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
	// ScoreWould is the session model id the public-score capability path
	// would pick (shadow unless DOWNSHIFT_SCORE_ROUTING=1). Model id only: no
	// prompt, no score value, not a reason code. Absent when the path
	// abstained or no score index is linked.
	ScoreWould string `json:"score_would,omitempty"`
	// ScoreDiffers is true when ScoreWould is not the model the catalog-tier
	// path leaves the child on. ScoreWouldCostRatio is the pick's catalog list
	// price over that model's (input plus output per 1M); absent when unknown.
	ScoreDiffers        bool    `json:"score_differs,omitempty"`
	ScoreWouldCostRatio float64 `json:"score_would_cost_ratio,omitempty"`
	// Confidence is reserved and never set: the router has no calibrated
	// probability, so the key stays absent rather than carry a made-up number.
	Confidence *float64 `json:"confidence,omitempty"`
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

// HasRealCost reports whether the event carries real provider costs.
// Both cost pointers must be non-nil; token counts alone are not enough.
func (e Event) HasRealCost() bool {
	return e.ActualCostUSD != nil && e.BaselineCostUSD != nil
}

// RealDeltaUSD returns baseline minus actual cost, signed: negative when the
// routed model cost more than the baseline (an overspend). Returns 0 when real
// costs are missing.
func (e Event) RealDeltaUSD() float64 {
	if !e.HasRealCost() {
		return 0
	}
	return *e.BaselineCostUSD - *e.ActualCostUSD
}

// RealSavedUSD returns baseline minus actual cost clamped at zero.
// Returns 0 when real costs are missing or when the routed model cost more.
// Use RealDeltaUSD when an overspend must stay visible.
func (e Event) RealSavedUSD() float64 {
	if delta := e.RealDeltaUSD(); delta > 0 {
		return delta
	}
	return 0
}

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

// ShadowRecommendation is advisory metadata emitted beside, never into, the
// deterministic hook rewrite. It contains no task text or provider selection.
type ShadowRecommendation struct {
	Tier              string   `json:"tier"`
	Confidence        float64  `json:"confidence"`
	Reasons           []string `json:"reasons"`
	RequiresReview    bool     `json:"requires_review"`
	BudgetConstrained bool     `json:"budget_constrained"`
	Apply             bool     `json:"apply"`
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

// EffortOrUnknown accepts only the cross-harness effort vocabulary.
func EffortOrUnknown(candidate string) string {
	if effortPattern.MatchString(candidate) {
		return candidate
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
)

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

// FromDecision builds an Event from a core.Decision and hook runtime context.
// It assumes a rewrite was emitted; callers set OutcomeAllow when the adapter
// returned the spawn unchanged.
// Self-correction fields are recorded only for held decisions, keeping clean
// events compact and old log readers unaffected.
func FromDecision(d core.Decision, correlationID, binaryVersion string) Event {
	ev := Event{
		CorrelationID:    correlationID,
		Timestamp:        time.Now().UTC().Format(time.RFC3339Nano),
		Source:           "hook",
		Agent:            "subagent",
		Harness:          d.Harness,
		Complexity:       d.Complexity.String(),
		FromModel:        ModelOrUnknown(d.CurrentModel.ID),
		ToModel:          ModelOrUnknown(d.Model.ID),
		RequestedEffort:  "unknown",
		FinalEffort:      EffortOrUnknown(d.Effort.String()),
		Verdict:          d.Verdict.String(),
		Tier:             d.Tier.String(),
		PolicyVersion:    PolicyVersion,
		BinaryVersion:    binaryVersion,
		Outcome:          OutcomeRewriteEmitted,
		EstimatedSavings: d.Savings,
		QuotaStatus:      d.QuotaStatus,
		QuotaSource:      d.QuotaSource,
		CreditHeld:       d.CreditHeld,
		PolicyFloor:      d.PolicyFloor,
		RoutingPolicy:    RoutingPolicy,
		ReasonCodes:      ReasonCodes(d),
	}
	if d.Verdict == core.VerdictUpshift {
		from := d.CurrentModel.InputM + d.CurrentModel.OutputM
		to := d.Model.InputM + d.Model.OutputM
		if from > 0 && to > from {
			ev.UpshiftCostRatio = to / from
		}
	}
	if d.ScoreWould != "" {
		ev.ScoreWould = ModelOrUnknown(d.ScoreWould)
		ev.ScoreDiffers = d.ScoreDiffers
		if r := d.ScoreWouldCostRatio; r > 0 && !math.IsInf(r, 0) && !math.IsNaN(r) {
			ev.ScoreWouldCostRatio = math.Round(r*1e4) / 1e4
		}
	}
	if len(d.Corrections) > 0 {
		ev.SafeVerdict = d.SafeVerdict.String()
		ev.Corrections = d.Corrections
	}
	if d.NBUpshift.Would {
		ev.NBUpshift = &NBUpshiftRecord{
			FromTier: d.NBUpshift.From.String(),
			ToTier:   d.NBUpshift.To.String(),
			Margin:   math.Round(d.NBUpshift.Margin*1e4) / 1e4,
			Applied:  d.NBUpshift.Applied,
		}
	}
	if d.NBDownshift.Would {
		// The opinion only exists for the Medium default, so it is mid -> small.
		ev.NBDownshift = &NBUpshiftRecord{
			FromTier: core.TierMid.String(),
			ToTier:   core.TierSmall.String(),
			Margin:   math.Round(d.NBDownshift.Margin*1e4) / 1e4,
			Applied:  d.NBDownshift.Applied,
		}
	}
	return ev
}

// Failure records a hook fail-open path without accepting raw input or errors
// that could contain prompt content.
func Failure(correlationID, binaryVersion, errorCode string) Event {
	return Event{
		CorrelationID: correlationID,
		Timestamp:     time.Now().UTC().Format(time.RFC3339Nano),
		Source:        "hook",
		Agent:         "subagent",
		PolicyVersion: PolicyVersion,
		BinaryVersion: binaryVersion,
		Outcome:       "error",
		ErrorCode:     errorCode,
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
		return fmt.Errorf("log rotation failed for %s: %w", path, err)
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
	return f.Sync() // flush to OS before releasing lock
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

	// RealSavedUSD is the sum of Event.RealSavedUSD over events with real cost.
	// RealCostEvents counts events where HasRealCost is true.
	RealSavedUSD   float64
	RealCostEvents int
	// RealCostInferred counts the RealCostEvents whose baseline was inferred
	// from settings, not requested by the harness.
	RealCostInferred int
	// RealCostDeltaUSD is the signed sum of Event.RealDeltaUSD (baseline minus
	// actual): negative when the router cost more than the baseline overall.
	// RealOverspendUSD is the sum of the negative deltas as a positive number,
	// the money spent above baseline on individual events.
	RealCostDeltaUSD float64
	RealOverspendUSD float64

	// ActualCost fields include catalog-priced observed spend even when the
	// requested baseline is unknown and no savings comparison can be made.
	ActualCostUSD    float64
	ActualCostEvents int

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
	// ScoreObserved counts decisions the capability path produced a pick for;
	// ScoreDiffers those where the pick is another model than the catalog
	// path leaves; ScoreRatioN and ScoreRatioSum average the pick's list price
	// over that model's price (events where both are known).
	ScoreObserved int
	ScoreDiffers  int
	ScoreRatioN   int
	ScoreRatioSum float64
	// ChildStatus counts how children ended (child_ended events), by status.
	ChildStatus map[ChildStatus]int
	// RespawnAfterFailure counts failed children followed by another spawn in
	// the same session within respawnWindow. A hint, not a verdict.
	RespawnAfterFailure int
	// CostBySession and CostBySubagent group observed catalog-priced cost by the
	// hashed session id and agent id carried on usage records.
	CostBySession  map[string]CostGroup
	CostBySubagent map[string]CostGroup

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
	// report does not print it as honor. HonorObserved counts decisions with
	// a linked resolved record whose rewrite_honored is true.
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
}

// addRealDelta folds one real-cost event into the clamped and signed totals.
func (s *Stats) addRealDelta(ev Event) {
	s.RealSavedUSD += ev.RealSavedUSD()
	delta := ev.RealDeltaUSD()
	s.RealCostDeltaUSD += delta
	if delta < 0 {
		s.RealOverspendUSD -= delta
	}
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

// Aggregate computes Stats from a slice of events.
func Aggregate(events []Event) Stats {
	s := Stats{ByComplexity: make(map[string]int), ChildStatus: make(map[ChildStatus]int), CostBySession: make(map[string]CostGroup), CostBySubagent: make(map[string]CostGroup), ReasonCodes: make(map[string]int), ModelMix: make(map[string]int), ByAgentType: make(map[string]AgentTypeCount)}
	// Build the index before counting: exported windows may arrive out of order.
	type linkKey struct {
		harness, decision string
	}
	decisions := make(map[linkKey]Event)
	// refuted holds rewrites a linked "resolved" record says the harness did
	// not run: they are not applied and save nothing.
	refuted := make(map[linkKey]bool)
	for _, ev := range events {
		if !IsCostOnlyOutcome(ev.Outcome) && ev.Outcome != "error" {
			decisions[linkKey{ev.Harness, DecisionKey(ev)}] = ev
		}
		if ev.Outcome == OutcomeResolved && ev.LinkedDecision != "" && ev.RewriteHonored != nil && !*ev.RewriteHonored {
			refuted[linkKey{ev.Harness, ev.LinkedDecision}] = true
		}
	}
	linked := make(map[linkKey]bool)
	for _, ev := range events {
		if ev.Outcome != OutcomeResolved && ev.Outcome != OutcomeBaseline && ev.ActualCostUSD != nil {
			s.ActualCostEvents++
			s.ActualCostUSD += *ev.ActualCostUSD
			if ev.Outcome == OutcomeUsage {
				addGroupCost(s.CostBySession, ev.SessionID, *ev.ActualCostUSD)
				addGroupCost(s.CostBySubagent, ev.AgentHash, *ev.ActualCostUSD)
			}
		}
		if ev.Outcome == OutcomeChildEnded && ev.ChildStatus != "" {
			s.ChildStatus[ev.ChildStatus]++
			continue // how a child ended: an observation, not a decision or cost
		}
		if ev.Outcome == OutcomeResolved {
			continue // an observation about an earlier decision, not a cost or a decision
		}
		if IsCostOnlyOutcome(ev.Outcome) {
			// Post-hoc cost records ("usage") and control-group events
			// ("baseline") are not decisions: they must not move Total,
			// verdict counts, complexity counts, or normalised units.
			// Usage records contribute real-dollar savings below.
			if ev.Outcome == OutcomeBaseline {
				s.Baseline++
				continue
			}
			s.UsageRecords++
			key := linkKey{ev.Harness, ev.LinkedDecision}
			identity := "event:" + DecisionKey(ev)
			if ev.AgentHash != "" {
				identity = "agent:" + ev.AgentHash
			}
			unique := linkKey{ev.Harness, identity}
			if decision, ok := decisions[key]; ok && ev.LinkedDecision != "" && !linked[unique] &&
				(ev.SessionID == "" || decision.SessionID == "" || ev.SessionID == decision.SessionID) {
				s.UsageLinked++
				linked[unique] = true
			}
			// A refuted rewrite saved nothing; an older record may still be
			// priced on the model it asked for, so its delta is not credited.
			if ev.HasRealCost() && !refuted[key] {
				s.RealCostEvents++
				s.addRealDelta(ev)
				if ev.BaselineSource == BaselineInferred || hasReason(decisions[key].ReasonCodes, ReasonBaselineFromSettings) {
					s.RealCostInferred++
				}
			}
			continue
		}
		s.Total++
		if ev.AgentType != "" {
			c := s.ByAgentType[ev.AgentType]
			c.Decisions++
			if ev.FromModel == "" || ev.FromModel == "unknown" {
				c.UnknownBaseline++
			}
			s.ByAgentType[ev.AgentType] = c
		}
		if ev.PolicyFloor {
			s.PolicyFloor++
		}
		s.ByComplexity[ev.Complexity]++
		isRefuted := refuted[linkKey{ev.Harness, DecisionKey(ev)}]
		ran := ev.ToModel
		if isRefuted {
			ran = ev.FromModel // the harness kept the requested model
		}
		if ran != "" && ran != "unknown" {
			s.ModelMix[ShortModel(ran)]++
		}
		if ev.ScoreWould != "" {
			s.ScoreObserved++
			if ev.ScoreDiffers {
				s.ScoreDiffers++
			}
			if ev.ScoreWouldCostRatio > 0 {
				s.ScoreRatioN++
				s.ScoreRatioSum += ev.ScoreWouldCostRatio
			}
		}
		for _, code := range ev.ReasonCodes {
			s.ReasonCodes[code]++
		}
		if strings.EqualFold(ev.Tier, core.TierFrontier.String()) {
			s.FrontierDecisions++
			if spentOnDoubt(ev) && !isRefuted {
				s.FrontierOnDoubt++
			}
		}
		applied := AppliedRewrite(ev.Outcome, ev.Corrections) && !isRefuted
		switch {
		case ev.Verdict == "DOWNSHIFT" && applied:
			s.Downshifted++
		case ev.Verdict == "UPSHIFT" && applied:
			s.Upshifted++
		case ev.Verdict == "DOWNSHIFT" || ev.Verdict == "UPSHIFT":
			s.NotApplied++
		case ev.Verdict == "OK":
			s.OK++
		default:
			s.Unknown++
		}
		// Normalised cost: baseline = 1.0 per event; routed = 1 - savings.
		// Only an applied downshift whose quota surface is available saves
		// anything. unknown and held are not credited routes. This is a
		// dimensionless proxy. Multiply by --cost-per-unit to get dollars.
		s.NormBaseline += 1.0
		if ev.Verdict == "DOWNSHIFT" && applied && SavingsCredit(ev) && ev.EstimatedSavings > 0 && ev.EstimatedSavings < 1 {
			s.NormRouted += 1 - ev.EstimatedSavings
		} else if ev.Verdict == "UPSHIFT" && applied && ev.UpshiftCostRatio > 1 {
			// An applied upshift costs more than the baseline. Charge the
			// known price ratio; an unknown ratio stays neutral (1.0).
			s.NormRouted += ev.UpshiftCostRatio
		} else {
			s.NormRouted += 1.0
		}
		// Real provider cost: only events with token usage contribute.
		if ev.HasRealCost() {
			s.RealCostEvents++
			s.addRealDelta(ev)
			if ev.BaselineSource == BaselineInferred || hasReason(ev.ReasonCodes, ReasonBaselineFromSettings) {
				s.RealCostInferred++
			}
		}
		// Self-correction: the verdict stands, the safe action applied.
		if len(ev.Corrections) > 0 {
			s.Corrected++
		}
	}
	s.RewriteShifted, s.RewriteHonored = CountInferredHonored(events)
	tallySpawnSurface(events, &s)
	s.ObservedChildTokens, s.ObservedChildUsage = ObservedChildTokens(events)
	s.RespawnAfterFailure = len(RespawnAfterFailure(events, respawnWindow))
	return s
}

// tallySpawnSurface counts observed honor and usage from linked port records,
// and quota from each decision's own status. It does not treat a later spawn
// or rewrite_emitted as honor.
func tallySpawnSurface(events []Event, s *Stats) {
	honor, usage := ObservedLinks(events)
	for _, ev := range events {
		if ev.Harness == "" || IsCostOnlyOutcome(ev.Outcome) || ev.Outcome == "error" {
			continue
		}
		surface := SurfaceFor(ev, honor[SurfaceKey(ev)], usage[SurfaceKey(ev)])
		switch surface.Honor {
		case HonorObserved:
			s.HonorObserved++
		default:
			s.HonorUnobserved++
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

// StatsOptions controls PrintStats output.
type StatsOptions struct {
	Days        int     // time window; <= 0 means all time
	CostPerUnit float64 // USD per normalised unit; 0 means show units only
	// Detail adds the full report (normalised units, honor, quota, usage, child
	// status, per-session cost, complexity breakdown) after the 3-line headline.
	Detail bool
}

// PrintStats writes a human-readable stats summary to w.
func PrintStats(events []Event, opts StatsOptions, w io.Writer) {
	filtered := FilterByDays(events, opts.Days)
	s := Aggregate(filtered)
	normSaved, normFrac := s.NormSaved()

	label := fmt.Sprintf("Last %d days", opts.Days)
	if opts.Days <= 0 {
		label = "All time"
	}

	printHeadline(w, s, opts.Days)
	fmt.Fprintf(w, "Estimated at list price, not an invoice. Measured against a control group: downshift report\n")
	if !opts.Detail {
		fmt.Fprintf(w, "\nFull report: downshift stats --detail\n")
		return
	}
	fmt.Fprintf(w, "\n%s\n", label)
	fmt.Fprintf(w, "─────────────────────────────────────\n")
	fmt.Fprintf(w, "Subagent decisions    %8d\n", s.Total)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Downshifted         %8d  %5.1f%%  (rewrite applied)\n", s.Downshifted, pct(s.Downshifted, s.Total))
	fmt.Fprintf(w, "  Upshifted           %8d  %5.1f%%  (rewrite applied)\n", s.Upshifted, pct(s.Upshifted, s.Total))
	fmt.Fprintf(w, "  Not applied         %8d  %5.1f%%  (classified shift, spawn unchanged: allow/held/blocked)\n", s.NotApplied, pct(s.NotApplied, s.Total))
	fmt.Fprintf(w, "  Unchanged (OK)      %8d  %5.1f%%\n", s.OK, pct(s.OK, s.Total))
	fmt.Fprintf(w, "  Unknown             %8d  %5.1f%%\n", s.Unknown, pct(s.Unknown, s.Total))
	if s.PolicyFloor > 0 {
		fmt.Fprintf(w, "  Raised by project floor %6d  %5.1f%%  (.downshift/policy.json)\n", s.PolicyFloor, pct(s.PolicyFloor, s.Total))
	}
	printChildStatus(w, s.ChildStatus)
	printAgentTypes(w, s.ByAgentType)
	if s.RespawnAfterFailure > 0 {
		fmt.Fprintf(w, "  Respawned after a failure  %5d  (hint only: may be unrelated work)\n", s.RespawnAfterFailure)
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "By complexity\n")
	for _, cls := range []string{"TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"} {
		n := s.ByComplexity[cls]
		fmt.Fprintf(w, "  %-8s            %8d  %5.1f%%\n", cls, n, pct(n, s.Total))
	}
	fmt.Fprintf(w, "\n")

	if s.Total > 0 {
		if opts.CostPerUnit > 0 {
			// Dollar mode: multiply normalised units by the user-supplied rate.
			baseline := s.NormBaseline * opts.CostPerUnit
			routed := s.NormRouted * opts.CostPerUnit
			saved := normSaved * opts.CostPerUnit
			fmt.Fprintf(w, "Estimated baseline    $%10.2f\n", baseline)
			fmt.Fprintf(w, "Estimated routed      $%10.2f\n", routed)
			fmt.Fprintf(w, "Estimated savings     $%10.2f  (%4.1f%%)\n", saved, normFrac*100)
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "Rate: $%.4f / unit  (--cost-per-unit=%.4f)\n", opts.CostPerUnit, opts.CostPerUnit)
		} else {
			// Unit mode: dimensionless proxy, no dollar claim.
			fmt.Fprintf(w, "Normalised baseline   %8.1f units\n", s.NormBaseline)
			fmt.Fprintf(w, "Normalised routed     %8.1f units\n", s.NormRouted)
			fmt.Fprintf(w, "Normalised savings    %8.1f units  (%4.1f%%)\n", normSaved, normFrac*100)
			fmt.Fprintf(w, "\n")
			fmt.Fprintf(w, "Note: 1 unit = cost of one unrouted event. Not real dollars.\n")
			fmt.Fprintf(w, "For dollar figures: downshift stats --cost-per-unit=<USD-per-unit>\n")
		}
	}
	if s.Corrected > 0 {
		fmt.Fprintf(w, "Safety-held           %8d  (guardrail applied, see corrections)\n", s.Corrected)
	}
	if s.ActualCostEvents > 0 {
		fmt.Fprintf(w, "Observed token cost   $%10.2f  (%d events, catalog pricing)\n", s.ActualCostUSD, s.ActualCostEvents)
		printCostGroups(w, s.CostBySession, s.CostBySubagent)
	}
	if s.ObservedChildUsage == 0 {
		fmt.Fprintf(w, "Observed child tokens unobserved\n")
	} else {
		fmt.Fprintf(w, "Observed child tokens %12d  (%d usage records)\n", s.ObservedChildTokens, s.ObservedChildUsage)
	}
	if s.RealCostEvents > 0 {
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "Real provider cost (%d events with token usage)\n", s.RealCostEvents)
		fmt.Fprintf(w, "Real saved            $%10.2f  (catalog list-price counterfactual)\n", s.RealSavedUSD)
		fmt.Fprintf(w, "Real cost delta       $%10.2f  (baseline minus actual; negative means the router cost more)\n", s.RealCostDeltaUSD)
		fmt.Fprintf(w, "Real overspend        $%10.2f  (events that cost more than their baseline)\n", s.RealOverspendUSD)
		if s.RealCostInferred > 0 {
			fmt.Fprintf(w, "Inferred baseline     %8d  of %d events: baseline guessed from settings (BASELINE_FROM_SETTINGS), not requested\n", s.RealCostInferred, s.RealCostEvents)
		}
	} else if s.ActualCostEvents > 0 {
		fmt.Fprintf(w, "Savings comparison: no events with a known requested baseline yet.\n")
	} else {
		fmt.Fprintf(w, "Real cost: no events with token usage yet (wire the PostToolUse hook to populate them).\n")
	}
	if s.UsageRecords > 0 {
		fmt.Fprintf(w, "Usage records         %8d  (all post-hoc usage records)\n", s.UsageRecords)
		fmt.Fprintf(w, "Usage linked          %8d  (distinct usage matched to decisions in this window)\n", s.UsageLinked)
	}
	if s.Baseline > 0 {
		fmt.Fprintf(w, "Baseline (no-route)   %8d  (control group, excluded from rates)\n", s.Baseline)
	}
	if s.Total > 0 {
		fmt.Fprintf(w, "Honor                 observed %d  unobserved %d\n", s.HonorObserved, s.HonorUnobserved)
		fmt.Fprintf(w, "Quota                 available %d  held %d  unknown %d\n", s.QuotaAvailable, s.QuotaHeld, s.QuotaUnknown)
		fmt.Fprintf(w, "Usage                 observed %d  unobserved %d\n", s.UsageObserved, s.UsageUnobserved)
		fmt.Fprintf(w, "Frontier decisions    %d  (rewritten on doubt, no deterministic lift: %d)\n", s.FrontierDecisions, s.FrontierOnDoubt)
		printScoreLine(w, s)
		if n := s.ReasonCodes[ReasonSemanticBoost]; n > 0 {
			fmt.Fprintf(w, "Semantic boost        %d of %d decisions (%.0f%%) were raised by the embedder, not a rule\n", n, s.Total, pct(n, s.Total))
		}
		if s.FrontierOnDoubt > 0 {
			fmt.Fprintf(w, "WARNING: %d frontier rewrite(s) were applied on an unconfident classification with no deterministic lift; check for silent overspend.\n", s.FrontierOnDoubt)
		}
	}
	fmt.Fprintf(w, "─────────────────────────────────────\n")
}

// printScoreLine reports the public-score capability path. It prints nothing
// when no decision carries a pick, so output without a score index is unchanged.
func printScoreLine(w io.Writer, s Stats) {
	if s.ScoreObserved == 0 {
		return
	}
	fmt.Fprintf(w, "Score pick differs    %d of %d decisions (%.0f%%)", s.ScoreDiffers, s.ScoreObserved, pct(s.ScoreDiffers, s.ScoreObserved))
	if s.ScoreRatioN > 0 {
		avg := s.ScoreRatioSum / float64(s.ScoreRatioN)
		switch {
		case avg < 1:
			fmt.Fprintf(w, ", list price %.0f%% lower on average", (1-avg)*100)
		case avg > 1:
			fmt.Fprintf(w, ", list price %.0f%% higher on average", (avg-1)*100)
		default:
			fmt.Fprintf(w, ", same list price on average")
		}
	}
	fmt.Fprintf(w, " (shadow unless DOWNSHIFT_SCORE_ROUTING=1)\n")
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}

// childStatusOrder is the fixed print order for "How children ended".
var childStatusOrder = []ChildStatus{ChildCompleted, ChildLaunched, ChildInterrupted, ChildError, ChildStopped, ChildUnknown}

// printChildStatus writes the "How children ended" section. It prints nothing
// when no child_ended events were counted, so output without them is unchanged.
func printChildStatus(w io.Writer, counts map[ChildStatus]int) {
	sum := 0
	for _, n := range counts {
		sum += n
	}
	if sum == 0 {
		return
	}
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "How children ended\n")
	for _, st := range childStatusOrder {
		if n := counts[st]; n > 0 {
			fmt.Fprintf(w, "  %-11s         %8d\n", st, n)
		}
	}
	fmt.Fprintf(w, "  Note: launched is a background start, not a success.\n")
}

// spentOnDoubt reports whether a frontier decision actually wrote the model on
// an unconfident classification with no deterministic lift. A decision a
// guardrail held, or one that left the spawn unchanged, spent nothing and is not
// counted: the alarm must only fire for money that was really spent.
func spentOnDoubt(ev Event) bool {
	return strings.EqualFold(ev.Tier, core.TierFrontier.String()) &&
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
		case ReasonRiskFloor, ReasonGraphEscalation, ReasonNBUpshiftApplied, ReasonTierFloor:
			return false
		}
	}
	return unconfident
}
