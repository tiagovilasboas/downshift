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
	Effort     Effort           // recommended reasoning intensity for this tier
	Intent     EscalationIntent // named task intent (Trivial/Normal/Review/Preserved)
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
}

// Shadow collects the observe-only opinions computed beside the routing.
type Shadow struct {
	// NBDownshift records the naive-Bayes TRIVIAL downshift opinion for
	// no-signal prompts. Always computed (shadow mode); it lowers the tier
	// only when DOWNSHIFT_NB_DOWNSHIFT=1.
	NBDownshift NBDownshift
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

// RaiseTier lifts the decision to at least tier t and recomputes everything
// derived from the tier: effort, recommended model, verdict against the current
// model, and the self-correction check. It never lowers a tier and reports
// whether it changed anything. currentModelID is the model in the payload, the
// same value passed to Route.
func (d *Decision) RaiseTier(t Tier, currentModelID string, r Resolver) bool {
	if t <= d.Tier {
		return false
	}
	d.Tier = t
	d.Effort = EffortFor(t)
	d.Model = resolveModel(d.Harness, t, r)
	d.Verdict, d.Baseline.CurrentModel = compareToCurrentModel(d.Harness, currentModelID, d.Model, r)
	d.Guards.PolicyFloor = true
	// A raised tier means any applied naive-Bayes downshift did not take effect.
	// Keep the opinion (Would) for telemetry, but do not claim it was applied.
	d.Shadow.NBDownshift.Applied = false
	d.Corrections = nil
	d.recheck()
	return true
}

// recheck runs the guardrails on the decision as it stands and records the
// safe verdict and the rules that corrected it. Route calls it once; any
// later step that moves the tier (the operator floor) calls it again.
func (d *Decision) recheck() {
	report := Check(*d)
	d.SafeVerdict = report.Safe
	d.Checked = true
	for _, v := range report.Violations {
		d.Corrections = append(d.Corrections, v.Rule)
	}
}

// resolveModel returns the catalog model for the given harness and tier.
// With a nil Resolver it returns a zero Model that only carries the tier.
func resolveModel(harness string, tier Tier, r Resolver) Model {
	if r != nil {
		return r.ModelFor(harness, tier)
	}
	// No resolver — return a zero Model with the tier set; caller handles gracefully.
	return Model{Tier: tier, Harness: harness}
}

// compareToCurrentModel looks up the current model and determines the verdict.
func compareToCurrentModel(harness, currentModelID string, recommended Model, r Resolver) (Verdict, Model) {
	if r == nil || currentModelID == "" {
		return VerdictUnknown, Model{}
	}

	current, known := r.LookupByID(harness, currentModelID)
	if !known {
		return VerdictUnknown, Model{}
	}

	switch {
	case current.Tier == recommended.Tier:
		return VerdictOK, current
	case current.Tier > recommended.Tier:
		return VerdictDownshift, current
	default:
		return VerdictUpshift, current
	}
}

// Summary renders a one-line, human-readable decision (the gearbox readout).
// When the escalation intent differs from the raw complexity label, it is
// included so operators can see when a code review was escalated to ReviewIntent
// rather than treated as a generic Complex construction task.
func (d Decision) Summary() string {
	intentSuffix := ""
	// Show intent only when it adds information not already in the complexity label.
	if d.Intent == ReviewIntent {
		intentSuffix = " [review]"
	} else if d.Intent == PreservedIntent {
		intentSuffix = " [preserved]"
	}

	switch d.Verdict {
	case VerdictDownshift:
		return fmt.Sprintf("%s task%s → downshift to %s (smaller model)",
			d.Complexity, intentSuffix, d.Model.ID)
	case VerdictUpshift:
		return fmt.Sprintf("%s task%s → upshift to %s (needs more torque)",
			d.Complexity, intentSuffix, d.Model.ID)
	case VerdictOK:
		return fmt.Sprintf("%s task%s → %s (right gear, no change)",
			d.Complexity, intentSuffix, d.Model.ID)
	default:
		return fmt.Sprintf("%s task%s → use %s (%s tier)",
			d.Complexity, intentSuffix, d.Model.ID, d.Tier)
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
