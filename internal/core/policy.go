// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/graphify"
)

// Tier maps a complexity to the minimum capable tier — the gearbox rule:
// which gear does this stretch of road need? Lives here because it is routing
// policy, not classification knowledge.
func (c Complexity) Tier() Tier {
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

// Decision is the full routing decision for one subagent task: what the task
// needs, what model to use on the given harness, and how it compares to the
// model the harness would have used by default.
type Decision struct {
	Complexity   Complexity
	Tier         Tier
	Effort       Effort           // recommended reasoning intensity for this tier
	Intent       EscalationIntent // named task intent (Trivial/Normal/Review/Preserved)
	Harness      string
	Model        Model // the model we recommend for this task
	CurrentModel Model // the model currently in effect (may be zero if unknown)
	// RequestedID is the model string from the hook payload, before catalog
	// normalisation. Adapters set it so a foreign harness id is not rewritten.
	RequestedID string
	Verdict     Verdict
	Savings     float64 // fraction cheaper vs current (0 if not cheaper / unknown)
	Confident   bool
	// Self-correction outcome, computed by Check inside Route. Verdict
	// keeps the classified value for diagnostics; SafeVerdict carries the
	// action adapters may apply. Checked is true for Route decisions and
	// false for hand-built ones, so ShouldRewriteModel stays backward
	// compatible with Decision literals that predate self-correction.
	SafeVerdict    Verdict
	Corrections    []string // guardrail rule IDs that held this decision, empty when clean
	Checked        bool
	GraphEscalated bool // true when graphify escalated the classification to Complex
	// RiskFloor is true when a risk signal (risk.go) lifted the task off the
	// small tier. Such a task is never downshifted and may be upshifted.
	RiskFloor bool
	// Risk lists the risk categories (risk.go) the prompt touches, whatever
	// its complexity. Guardrail R5 holds any downshift of such a task.
	Risk []string
	// NBUpshift records the naive-Bayes upshift-only second opinion. It is
	// always computed (shadow mode); it changes the tier only when
	// DOWNSHIFT_NB_UPSHIFT=1, and then only upward.
	NBUpshift NBUpshift
	// NBDownshift records the naive-Bayes TRIVIAL downshift opinion for
	// no-signal prompts. Always computed (shadow mode); it lowers the tier
	// only when DOWNSHIFT_NB_DOWNSHIFT=1.
	NBDownshift NBDownshift
	// SessionUnknown is set by adapters when no session allowlist could be
	// resolved (no hook list, no session-models.json entry). The hook then
	// never rewrites; runners use it to warn the operator once per call.
	SessionUnknown bool
	// QuotaStatus is provider budget evidence for the proposed/retained model.
	// unknown is explicit, including legacy routing without a quota source.
	QuotaStatus string
	QuotaSource string
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

// Route is the main entry point: given a task prompt, a harness, the current
// model id (may be empty), and an optional Resolver, produce a routing Decision.
//
// When r is nil, Route falls back to the in-package Catalog for backward
// compatibility. Once all callers pass a real Resolver (Wave 4.4), the Catalog
// var will be removed from core.
func Route(prompt, harness, currentModelID string, r ...Resolver) Decision {
	var res Resolver
	if len(r) > 0 && r[0] != nil {
		res = r[0]
	}

	tier, cls, graphEscalated, nbUp, nbDown := classifyTask(prompt)
	effort := EffortFor(tier)
	recommended := resolveModel(harness, tier, res)
	verdict, savings, current := compareToCurrentModel(harness, currentModelID, recommended, res)

	d := Decision{
		Complexity:     cls.Complexity,
		Tier:           tier,
		Effort:         effort,
		Harness:        harness,
		Model:          recommended,
		CurrentModel:   current,
		Verdict:        verdict,
		Savings:        savings,
		Confident:      cls.Confident,
		GraphEscalated: graphEscalated,
		RiskFloor:      cls.RiskFloor,
		Risk:           RiskCategories(prompt),
		NBUpshift:      nbUp,
		NBDownshift:    nbDown,
	}
	d.Intent = IntentFor(prompt, cls, d, res)
	// Self-correction validates the decision against the guardrails before
	// any adapter acts. The classified verdict is preserved for telemetry;
	// the safe action is exposed separately.
	report := Check(d)
	d.SafeVerdict = report.Safe
	d.Checked = true
	for _, v := range report.Violations {
		d.Corrections = append(d.Corrections, v.Rule)
	}
	return d
}

// classifyTask classifies the prompt once (regex, semantic boost, risk floor,
// graphify escalation) and returns the target tier, the final
// classification and whether graphify escalated it. Route uses this single
// result for complexity, confidence and intent so they never disagree.
// The naive-Bayes second opinion runs last, on the final tier.
func classifyTask(prompt string) (Tier, Classification, bool, NBUpshift, NBDownshift) {
	cls := classifyWithSemantic(prompt, Classify(prompt))
	escalated := false

	// Graphify escalation: if the text classifier is uncertain but the prompt
	// mentions critical files/symbols, escalate to Complex for safer handling.
	// A confident classification is left alone; Complex needs no escalation.
	if cls.Complexity != Complex && !cls.Confident {
		hint := graphifyHint(prompt)
		if hint.ShouldEscalate {
			cls.Complexity = Complex
			escalated = true
		}
	}

	nbDown := nbTrivialOpinion(prompt, cls, escalated)
	if nbDown.Applied {
		cls.Complexity = Trivial
		cls.Confident = false
	}
	nbUp := nbSecondOpinion(prompt, cls.Complexity.Tier())
	if nbUp.Applied {
		cls.Complexity = complexityForTier(nbUp.To)
		cls.Confident = false
	}
	return cls.Complexity.Tier(), cls, escalated, nbUp, nbDown
}

// resolveModel returns the catalog model for the given harness and tier.
// Uses the Resolver when provided; panics if nil (callers must inject one).
func resolveModel(harness string, tier Tier, r Resolver) Model {
	if r != nil {
		return r.ModelFor(harness, tier)
	}
	// No resolver — return a zero Model with the tier set; caller handles gracefully.
	return Model{Tier: tier, Harness: harness}
}

// compareToCurrentModel looks up the current model and determines the verdict.
func compareToCurrentModel(harness, currentModelID string, recommended Model, r Resolver) (Verdict, float64, Model) {
	if r == nil || currentModelID == "" {
		return VerdictUnknown, 0, Model{}
	}

	current, known := r.LookupByID(harness, currentModelID)
	if !known {
		return VerdictUnknown, 0, Model{}
	}

	switch {
	case current.Tier == recommended.Tier:
		return VerdictOK, 0, current
	case current.Tier > recommended.Tier:
		return VerdictDownshift, r.SavingsRatio(current, recommended), current
	default:
		return VerdictUpshift, 0, current
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
		return fmt.Sprintf("%s task%s → downshift to %s (~%.0f%% cheaper)",
			d.Complexity, intentSuffix, d.Model.ID, d.Savings*100)
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

// ShouldRewriteModel reports whether an adapter should replace the harness
// model input. It is deliberately harness-agnostic: every adapter receives
// the same behavior for upshifts, downshifts, and missing source models.
//
// The gate consults the self-corrected action, so a decision held by a
// guardrail is never applied even when the classified verdict suggests a
// rewrite. Hand-built Decisions without a Check report fall back to the
// classified verdict for backward compatibility.
func (d Decision) ShouldRewriteModel() bool {
	if d.Model.ID == "" {
		return false
	}
	action := d.Verdict
	if d.Checked {
		action = d.SafeVerdict
	}
	// An uncertain classifier result must not spend money in either direction.
	// A downgrade stays put so a hard task is not underpowered. An upshift
	// stays put so a small model is not replaced by a dearer tier on a weak
	// signal. Confident rewrites still apply. Plan falls back when this
	// returns false.
	// Exception: a risk-floored or graphify-escalated task may move up — both
	// are deterministic rules, not weak signals — and so may an NB upshift the
	// user opted into with DOWNSHIFT_NB_UPSHIFT=1.
	if action == VerdictUpshift && (d.RiskFloor || d.GraphEscalated || d.NBUpshift.Applied) {
		return true
	}
	if (action == VerdictDownshift || action == VerdictUpshift) && !d.Confident {
		return false
	}
	// No current model known: the child would inherit the harness default
	// (often the parent's model). An unconfident small-tier recommendation
	// must not be written over that default (guardrail R6). Mid and above
	// may still be written: they never move a task below the mid tier.
	if action == VerdictUnknown && unconfidentUnknownBelowMid(d) {
		return false
	}
	return action == VerdictDownshift || action == VerdictUpshift || action == VerdictUnknown
}

// ShouldApplyEffort reports whether a harness with native effort controls
// should inject the selected effort. This includes VerdictOK so a child does
// not accidentally inherit a lower effort from a same-tier parent.
func (d Decision) ShouldApplyEffort() bool {
	return d.Model.ID != ""
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

// graphifyHint uses DOWNSHIFT_GRAPHIFY_CMD when set; otherwise offline (nil fetcher).
func graphifyHint(prompt string) graphify.EscalationHint {
	return graphify.Hint(prompt, graphify.DefaultCriteria(), graphify.FetcherFromEnv())
}
