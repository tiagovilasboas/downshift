// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
)

// TierOf maps a complexity to the minimum capable tier — the gearbox rule:
// which gear does this stretch of road need? It lives in the route policy,
// not in the classifier.
func TierOf(c Complexity) Tier {
	switch c {
	case Trivial:
		return TierSmall // flat straight: high gear, cheap
	case Simple:
		return TierSmall // one isolated change stays on the cheap tier
	case Medium:
		return TierMid
	case Complex:
		return TierFrontier // sharp curve: downshift for torque
	default:
		return TierMid
	}
}

// RoutingNote mirrors routing/contract.Note without its headroom fields. It
// holds enums, tiers and declared ids only.
type RoutingNote struct {
	Verdict, Reason, RuleID, Profile, TierFrom, TierTo string
}

// Decision is the full routing decision for one subagent task: what the task
// needs, what model to use on the given harness, and how it compares to the
// model the harness would have used by default.
type Decision struct {
	Complexity Complexity
	Tier       Tier
	Effort     Effort // recommended reasoning intensity for this tier
	Harness    string
	Model      Model // the model we recommend for this task
	Verdict    Verdict
	Confident  bool
	// Self-correction outcome, computed by Check inside Route. Verdict
	// keeps the classified value for diagnostics; SafeVerdict carries the
	// action adapters may apply. Checked is true for Route decisions and
	// false for hand-built ones; selectSessionTarget only honours a
	// SafeVerdict of OK when Checked is true.
	SafeVerdict Verdict
	Corrections []string // guardrail rule IDs that held this decision, empty when clean
	Checked     bool
	// Reason names why routing v2 held or refused a move (a
	// routing/contract.Reason value as a string); empty when nothing was held.
	Reason string
	// Note is the routing note of the decision as plain strings (verdict,
	// declared rule or profile id, tiers), so the event log can say why a spawn
	// changed or was kept. Empty on hand-built decisions.
	Note RoutingNote
	// Advice is an enum for the orchestrator when nothing moved but the task
	// looks harder than the tier it asked for (AdviceComplexOnSmall); "" otherwise.
	Advice string
	// AgentPin names the agent and the two tiers when Guards.AboveAgentDefault
	// is set. Ids and enums only.
	AgentPin AgentPin
	// Risk lists the risk categories (risk.go) the prompt touches, whatever
	// its complexity. Guardrail R5 holds any downshift of such a task.
	Risk []string
	// NoSignal is true when the classifier matched no content signal and the
	// complexity is only the safe Medium default (Classification.NoSignal).
	NoSignal bool
	// TaskNotVisible is true when the task text is opaque (encrypted or
	// inaccessible) and routing cannot be based on content classification.
	TaskNotVisible bool
	// UnsupportedLanguage records the language guard without persisting text.
	UnsupportedLanguage bool

	// Baseline describes the model the harness would have used by default and
	// how reliably it was observed.
	Baseline Baseline
	// Session carries the session-scope facts adapters resolve for the write:
	// whether the allowlist is known and the provider budget evidence.
	Session Session
	// Guards records the deterministic interventions that moved the tier or
	// cleared confidence after classification.
	Guards Guards
	// Shadow holds the observe-only second opinions. None of them changes the
	// routing unless its own opt-in flag is set.
	Shadow Shadow
}

// Baseline is the model currently in effect for the task and the provenance
// of that observation.
type Baseline struct {
	CurrentModel Model // the model currently in effect (may be zero if unknown)
	// RequestedID is the model string from the hook payload, before catalog
	// normalisation. Adapters set it so a foreign harness id is not rewritten.
	RequestedID string
	// FromSettings is true when the baseline (CurrentModel) was not seen
	// in the payload or transcript but inferred from the session's configured
	// model. A `--model` flag or a mid-session /model is invisible to the hook,
	// so such a baseline can be wrong.
	FromSettings bool
	// Guessed is true when the adapter inferred the baseline (Claude
	// Code settings, Cursor parent model_id) instead of reading it from the call.
	Guessed bool
	// GuessHeld is true when an adapter held a downshift because its baseline
	// was guessed (Claude Code settings, Cursor parent model_id) and the target
	// was not the small tier, which is the only one surely cheaper.
	GuessHeld bool
	// Pinned is true when the harness's agent definition fixes the child's model
	// (an agent file or a built-in type that names one). The model is then the
	// owner's explicit choice: the baseline is known, and routing never rewrites it.
	Pinned bool
}

// Session is the session-scope evidence behind the write decision.
type Session struct {
	// Unknown is set by adapters when no session allowlist could be
	// resolved (no hook list, no session-models.json entry). The hook then
	// never rewrites; runners use it to warn the operator once per call.
	Unknown bool
	// QuotaStatus is provider budget evidence for the proposed/retained model.
	// unknown is explicit, including legacy routing without a quota source.
	QuotaStatus string
	QuotaSource string
	// CreditHeld is true when the credit gate refused this rewrite. A
	// guardrail hold does not set it. Exhausted and stale status are a hold
	// on their own; this flag covers unknown when a present source or
	// required mode refused the write.
	CreditHeld bool
}

// Guards records which deterministic rule moved the decision after the
// classifier scored the task.
type Guards struct {
	// RiskFloor is true when a risk signal (risk.go) lifted the task off the
	// small tier. Such a task is never downshifted and may be upshifted.
	RiskFloor bool
	// RiskAbstain is true when risk rules cleared Confident without a tier lift.
	RiskAbstain bool
	// PolicyFloor is true when an operator tier floor (internal/policyfile)
	// raised Tier above what the classifier chose.
	PolicyFloor bool
	// SemanticLift is true when the semantic boost, not a regex rule, raised
	// the complexity. Telemetry records it so non-deterministic lifts stay visible.
	SemanticLift bool
	// TaskFocus is true when the actionable task of a long brief, with the
	// rest scored as context, lowered the class or changed its confidence.
	TaskFocus bool
	// AgentTypeHint is true when the subagent type, not the task text, set the
	// tier (RouteAgent): the text read was unconfident and the type's hint won.
	AgentTypeHint bool
	// AgentDefaultTier is true when the agent definition's declared tier, not
	// the inherited model, set the tier (the call named no model).
	AgentDefaultTier bool
	// AboveAgentDefault is true when the call pinned a model above the tier the
	// agent definition declares (see Decision.AgentPin).
	AboveAgentDefault bool
}

// Shadow collects the observe-only opinions computed beside the routing.
type Shadow struct {
	// AdaptWould is the tier local memory would select. It is shadow-only:
	// Route does not copy it into Tier until an independent eval promotes it.
	// AdaptWouldSet is false when memory is absent or agrees with Tier.
	AdaptWould    Tier
	AdaptWouldSet bool
	// GatewayWould is the id of the allowlisted gateway row that would
	// serve this decision's tier, or empty when there is no allowlist or no
	// row of that tier. Shadow only: it never changes the routed model.
	GatewayWould string
}

// Verdict tells the caller what to do about the current model.
type Verdict int

const (
	// VerdictOK — the current model already matches the needed tier.
	VerdictOK Verdict = iota
	// VerdictDownshift — the current model is stronger (and pricier) than needed.
	VerdictDownshift
	// VerdictUpshift — the current model is weaker than the task needs.
	VerdictUpshift
	// VerdictUnknown — no current model known; we only state the recommendation.
	VerdictUnknown
)

// String returns the human-readable verdict.
func (v Verdict) String() string {
	switch v {
	case VerdictOK:
		return "OK"
	case VerdictDownshift:
		return "DOWNSHIFT"
	case VerdictUpshift:
		return "UPSHIFT"
	default:
		return "UNKNOWN"
	}
}

// Summary renders a one-line, human-readable decision (the gearbox readout).
func (d Decision) Summary() string {
	switch d.Verdict {
	case VerdictDownshift:
		return fmt.Sprintf("%s task → downshift to %s (smaller model)", d.Complexity, d.Model.ID)
	case VerdictUpshift:
		return fmt.Sprintf("%s task → upshift to %s (needs more torque)", d.Complexity, d.Model.ID)
	case VerdictOK:
		return fmt.Sprintf("%s task → %s (right gear, no change)", d.Complexity, d.Model.ID)
	default:
		return fmt.Sprintf("%s task → use %s (%s tier)", d.Complexity, d.Model.ID, d.Tier)
	}
}

// ShouldPreserveExplicitModel returns true when the user's current model is
// marked routing:"explicit_only" in the catalog. Explicit-only models (e.g.
// a top-tier or specialised model the user deliberately selected) must never
// be replaced by automatic routing — the adapter should leave them unchanged.
//
// When the current model ID is empty or not found in the catalog, the method
// returns false so normal routing proceeds.
func (d Decision) ShouldPreserveExplicitModel(currentModelID string, r Resolver) bool {
	if currentModelID == "" || r == nil {
		return false
	}
	return r.IsExplicitOnly(d.Harness, currentModelID)
}
