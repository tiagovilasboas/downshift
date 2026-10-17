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
	Complexity Complexity
	Tier       Tier
	Effort     Effort           // recommended reasoning intensity for this tier
	Intent     EscalationIntent // named task intent (Trivial/Normal/Review/Preserved)
	Harness    string
	Model      Model // the model we recommend for this task
	Verdict    Verdict
	Savings    float64 // fraction cheaper vs current (0 if not cheaper / unknown)
	Confident  bool
	// Self-correction outcome, computed by Check inside Route. Verdict
	// keeps the classified value for diagnostics; SafeVerdict carries the
	// action adapters may apply. Checked is true for Route decisions and
	// false for hand-built ones; selectSessionTarget only honours a
	// SafeVerdict of OK when Checked is true.
	SafeVerdict Verdict
	Corrections []string // guardrail rule IDs that held this decision, empty when clean
	Checked     bool
	// Risk lists the risk categories (risk.go) the prompt touches, whatever
	// its complexity. Guardrail R5 holds any downshift of such a task.
	Risk []string
	// NoSignal is true when the classifier matched no content signal and the
	// complexity is only the safe Medium default (Classification.NoSignal).
	NoSignal bool
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
	// E5Fuse is the trace of the opt-in multilingual fuse that ran inside the
	// classification (DOWNSHIFT_E5_FUSE=1): what the helper answered and what
	// the fuse did with it. Zero when the flag is off.
	E5Fuse E5Fuse
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
	RiskAbstain    bool
	GraphEscalated bool // true when graphify escalated the classification to Complex
	// PolicyFloor is true when an operator tier floor (internal/policyfile)
	// raised Tier above what the classifier chose.
	PolicyFloor bool
	// SemanticLift is true when the semantic boost, not a regex rule, raised
	// the complexity. Telemetry records it so non-deterministic lifts stay visible.
	SemanticLift bool
}

// Shadow collects the observe-only opinions computed beside the routing.
type Shadow struct {
	// NBUpshift records the naive-Bayes upshift-only second opinion. It is
	// always computed (shadow mode); it changes the tier only when
	// DOWNSHIFT_NB_UPSHIFT=1, and then only upward.
	NBUpshift NBUpshift
	// NBDownshift records the naive-Bayes TRIVIAL downshift opinion for
	// no-signal prompts. Always computed (shadow mode); it lowers the tier
	// only when DOWNSHIFT_NB_DOWNSHIFT=1.
	NBDownshift NBDownshift
	// AdaptWould is the tier local memory would select. It is shadow-only:
	// Route does not copy it into Tier until an independent eval promotes it.
	// AdaptWouldSet is false when memory is absent or agrees with Tier.
	AdaptWould    Tier
	AdaptWouldSet bool
	// ScoreWould is the session model id the public-score capability path
	// (score_select.go) would pick for this task, or empty when it abstained.
	// Shadow by default: it changes the routing only when ScoreApplied is set.
	// ScoreWouldCostRatio is the list price of that pick over the price of the
	// model the catalog-tier path leaves the child on (0 when either price is
	// unknown). ScoreDiffers is true when the pick is a different model.
	ScoreWould          string
	ScoreDiffers        bool
	ScoreWouldCostRatio float64
	// ScoreApplied is true when DOWNSHIFT_SCORE_ROUTING=1 and the plan
	// rewrote the model to ScoreWould.
	ScoreApplied bool
	// E5Shadow holds a parallel multilingual embedding observation when
	// DOWNSHIFT_E5_SHADOW=1. It never changes Tier or Complexity by itself.
	// Production fuse is a separate flag (DOWNSHIFT_E5_FUSE=1) in ClassifyWithSemantic.
	E5Shadow E5Shadow
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
// Without a Resolver the decision carries tiers but no model ids: resolveModel
// returns a zero Model and the verdict stays Unknown, so the caller fails open.
// Hooks always pass the catalog; offline commands may not.
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
		Complexity:          cls.Complexity,
		Tier:                tier,
		Effort:              effort,
		Harness:             harness,
		Model:               recommended,
		Verdict:             verdict,
		Savings:             savings,
		Confident:           cls.Confident,
		Risk:                RiskCategories(prompt),
		NoSignal:            cls.NoSignal,
		UnsupportedLanguage: cls.UnsupportedLanguage,
		Baseline: Baseline{
			CurrentModel: current,
		},
		Guards: Guards{
			GraphEscalated: graphEscalated,
			RiskFloor:      cls.RiskFloor,
			RiskAbstain:    cls.RiskAbstain,
			SemanticLift:   cls.SemanticLift,
		},
		Shadow: Shadow{
			NBUpshift:   nbUp,
			NBDownshift: nbDown,
		},
		E5Fuse: cls.E5Fuse,
	}
	applyE5Shadow(prompt, cls, &d)
	d.Intent = IntentFor(prompt, cls, d, res)
	// Self-correction validates the decision against the guardrails before
	// any adapter acts. The classified verdict is preserved for telemetry;
	// the safe action is exposed separately.
	d.recheck()
	// The operator floor runs before adapt memory so the shadow comparison
	// sees the final tier.
	applyTierFloor(&d, currentModelID, res)
	applyAdaptMemory(prompt, &d, res)
	return d
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
	d.Verdict, d.Savings, d.Baseline.CurrentModel = compareToCurrentModel(d.Harness, currentModelID, d.Model, r)
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
// With a nil Resolver it returns a zero Model that only carries the tier.
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
