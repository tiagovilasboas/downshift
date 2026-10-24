// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

// agentTypeHintTier decides whether the subagent type supplies the tier for a
// spawn whose text read was in doubt. The hint is data
// (HarnessCapabilities.AgentTypeHints); this function is the only rule.
//
// The hint fills doubt, it never overrides evidence:
//   - the text classifier must be unconfident (a confident read always wins,
//     whether it is lower or higher than the hint);
//   - no risk signal of any kind, and no deterministic lift (NB upshift),
//     semantic lift or unreadable language: those stay as classified;
//   - an unconfident COMPLEX read is kept as doubt, not lowered by a name;
//   - the current model must be known, and the hint is capped at its tier, so
//     the hint can never produce an upshift above the model already in effect.
//
// Pinned built-ins (claude-code-guide, statusline-setup) carry no hint and
// stay pinned through Baseline.Pinned.
func agentTypeHintTier(harness, agentType, prompt string, textTier Tier, cls Classification, nbUp NBUpshift, currentModelID string, res Resolver) (Tier, bool) {
	if agentType == "" || res == nil || currentModelID == "" {
		return textTier, false
	}
	caps, ok := CapsFor(harness)
	if !ok {
		return textTier, false
	}
	hint, ok := caps.AgentTypeHints[agentType]
	if !ok {
		return textTier, false
	}
	if cls.Confident || cls.Complexity == Complex || cls.RiskFloor || cls.RiskAbstain ||
		cls.SemanticLift || cls.UnsupportedLanguage || nbUp.Applied || len(RiskCategories(prompt)) > 0 {
		return textTier, false
	}
	current, known := res.LookupByID(harness, currentModelID)
	if !known {
		return textTier, false
	}
	if hint > current.Tier {
		hint = current.Tier
	}
	return hint, true
}
