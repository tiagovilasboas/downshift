// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

// Package telemetry records routing decisions to a local JSONL file and
// renders aggregated stats. All data stays on the user's machine — nothing
// is sent to any external service.
//
// Privacy note: prompt contents are never stored. Each event contains only
// routing metadata (harness, complexity class, model IDs, verdict, and a
// normalised savings fraction). This makes the event log safe for corporate
// environments where task prompts may contain sensitive information.
//
// Event log location: ~/.harness-downshift/events.jsonl
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
	"sync"
	"time"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
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

	// Optional real-cost fields (nil = provider usage not tracked for this event).
	// When absent, fall back to the normalised estimate above; never fake dollars.
	InputTokens     *int64   `json:"input_tokens,omitempty"`      // provider-reported input tokens
	OutputTokens    *int64   `json:"output_tokens,omitempty"`     // provider-reported output tokens
	CachedTokens    *int64   `json:"cached_tokens,omitempty"`     // provider-reported cached input tokens
	ActualCostUSD   *float64 `json:"actual_cost_usd,omitempty"`   // real routed cost in USD
	BaselineCostUSD *float64 `json:"baseline_cost_usd,omitempty"` // real baseline cost in USD

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

// RealSavedUSD returns baseline minus actual cost clamped at zero.
// Returns 0 when real costs are missing or when the routed model cost more.
func (e Event) RealSavedUSD() float64 {
	if !e.HasRealCost() {
		return 0
	}
	saved := *e.BaselineCostUSD - *e.ActualCostUSD
	if saved < 0 {
		return 0
	}
	return saved
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

// defaultEventPath returns ~/.harness-downshift/events.jsonl.
func defaultEventPath() string {
	if path := os.Getenv("DOWNSHIFT_EVENT_LOG"); path != "" {
		return path
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".harness-downshift", "events.jsonl")
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

	// UsageLinked counts post-hoc cost records (outcome "usage").
	// Baseline counts control-group events recorded with routing disabled
	// (outcome "baseline", see DOWNSHIFT_NO_ROUTE): classified but never
	// applied, excluded from decision counts so before/after comparison
	// stays honest.
	UsageLinked int
	Baseline    int

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

	// RewriteShifted counts rewrite_emitted events that asked for a different model.
	// RewriteHonored counts those later confirmed: explicit rewrite_honored, or
	// the next same-session spawn arriving with requested_model equal to the
	// previous final_model. Inference never writes the log.
	RewriteShifted int
	RewriteHonored int
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

// Aggregate computes Stats from a slice of events.
func Aggregate(events []Event) Stats {
	s := Stats{ByComplexity: make(map[string]int)}
	for _, ev := range events {
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
			s.UsageLinked++
			if ev.HasRealCost() {
				s.RealCostEvents++
				s.RealSavedUSD += ev.RealSavedUSD()
			}
			continue
		}
		s.Total++
		s.ByComplexity[ev.Complexity]++
		applied := AppliedRewrite(ev.Outcome, ev.Corrections)
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
		// Only an applied downshift saves anything. This is a dimensionless
		// proxy. Multiply by --cost-per-unit to get dollars.
		s.NormBaseline += 1.0
		if ev.Verdict == "DOWNSHIFT" && applied && ev.EstimatedSavings > 0 && ev.EstimatedSavings < 1 {
			s.NormRouted += 1 - ev.EstimatedSavings
		} else {
			s.NormRouted += 1.0
		}
		// Real provider cost: only events with token usage contribute.
		if ev.HasRealCost() {
			s.RealCostEvents++
			s.RealSavedUSD += ev.RealSavedUSD()
		}
		// Self-correction: the verdict stands, the safe action applied.
		if len(ev.Corrections) > 0 {
			s.Corrected++
		}
	}
	s.RewriteShifted, s.RewriteHonored = CountInferredHonored(events)
	return s
}

// StatsOptions controls PrintStats output.
type StatsOptions struct {
	Days        int     // time window; <= 0 means all time
	CostPerUnit float64 // USD per normalised unit; 0 means show units only
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

	fmt.Fprintf(w, "%s\n", label)
	fmt.Fprintf(w, "─────────────────────────────────────\n")
	fmt.Fprintf(w, "Subagent decisions    %8d\n", s.Total)
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Downshifted         %8d  %5.1f%%  (rewrite applied)\n", s.Downshifted, pct(s.Downshifted, s.Total))
	fmt.Fprintf(w, "  Upshifted           %8d  %5.1f%%  (rewrite applied)\n", s.Upshifted, pct(s.Upshifted, s.Total))
	fmt.Fprintf(w, "  Not applied         %8d  %5.1f%%  (classified shift, spawn unchanged: allow/held/blocked)\n", s.NotApplied, pct(s.NotApplied, s.Total))
	fmt.Fprintf(w, "  Unchanged (OK)      %8d  %5.1f%%\n", s.OK, pct(s.OK, s.Total))
	fmt.Fprintf(w, "  Unknown             %8d  %5.1f%%\n", s.Unknown, pct(s.Unknown, s.Total))
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
	if s.RealCostEvents > 0 {
		fmt.Fprintf(w, "\n")
		fmt.Fprintf(w, "Real provider cost (%d events with token usage)\n", s.RealCostEvents)
		fmt.Fprintf(w, "Real saved            $%10.2f\n", s.RealSavedUSD)
	} else {
		fmt.Fprintf(w, "Real cost: no events with token usage yet (wire the PostToolUse hook to populate them).\n")
	}
	if s.UsageLinked > 0 {
		fmt.Fprintf(w, "Usage linked          %8d  (PostToolUse real-cost records)\n", s.UsageLinked)
	}
	if s.Baseline > 0 {
		fmt.Fprintf(w, "Baseline (no-route)   %8d  (control group, excluded from rates)\n", s.Baseline)
	}
	if s.RewriteShifted > 0 {
		fmt.Fprintf(w, "Rewrite honored (inferred) %3d  / %d applied shifts (harness-reported resolvedModel matched, or a later same-session spawn asked for the written model; the child's billed model is still not read)\n", s.RewriteHonored, s.RewriteShifted)
	}
	fmt.Fprintf(w, "─────────────────────────────────────\n")
}

func pct(n, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(n) / float64(total) * 100
}
