// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// v1Decider is the classifier-driven policy: regex and semantic
// classification, the agent-type hint, the self-correction rules, the
// operator floor and adapt memory. It is the default Decider.
type v1Decider struct{}

// Decide routes the request with the v1 policy. The agent type is a
// classifier signal only (see applyAgentTypeHint): an empty or unmapped type
// routes on the prompt alone.
func (v1Decider) Decide(req Request) Decision {
	prompt, harness, agentType, currentModelID, res := req.Prompt, req.Harness, req.AgentType, req.CurrentModelID, req.Resolver

	tier, cls, nbUp, nbDown := classifyTask(prompt)
	if economyEnv() && cls.Confident && !cls.RiskFloor && len(RiskCategories(prompt)) == 0 && tier > TierSmall {
		tier--
		cls.Complexity = complexityForTier(tier)
	}
	hinted := false
	if t, ok := agentTypeHintTier(harness, agentType, prompt, tier, cls, nbUp, currentModelID, res); ok {
		tier, hinted = t, true
		cls.Complexity, cls.Confident = complexityForTier(t), true
		nbDown.Applied = false
	}
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
			RiskFloor:     cls.RiskFloor,
			RiskAbstain:   cls.RiskAbstain,
			SemanticLift:  cls.SemanticLift,
			TaskFocus:     cls.TaskFocus,
			AgentTypeHint: hinted,
		},
		Shadow: Shadow{
			NBUpshift:   nbUp,
			NBDownshift: nbDown,
		},
	}
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
