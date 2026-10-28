// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"strings"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// DefaultProfile is one row of the default profile table shipped with the
// binary (roadmap N11). It applies only when nothing declared in policy.json
// matched the spawn, so a project or user file always wins. Tiers are
// abstract: each harness maps them to its own models through catalog.json.
//
// A row whose Tier is TierUnknown keeps the requested model; it is listed so
// the table states that choice instead of leaving it implicit. RiskTier is the
// tier the row asks for when the task touches a risk category; it is never
// small. The guard still has the last word, and it keeps the requested tier on
// any risk, so RiskTier never moves a spawn below what the guard allows.
//
// ClassTier applies to a keeping row only: when the classifier is confident
// that the task is below COMPLEX and no risk category matched, the row asks
// for ClassTier instead of keeping. A catch-all type then stops inheriting the
// main session's frontier model for work the classifier rates as routine. It
// is never small: a catch-all carries every tool and the classifier reads the
// text only.
type DefaultProfile struct {
	Harness   contract.HarnessID
	AgentType string
	Tier      contract.Tier
	RiskTier  contract.Tier
	ClassTier contract.Tier
}

// Keeps reports whether the row leaves the requested model in place.
func (d DefaultProfile) Keeps() bool { return !d.Tier.Valid() }

// TierFor returns the tier the row asks for, given whether the task touches
// a risk category.
func (d DefaultProfile) TierFor(risky bool) contract.Tier {
	if risky {
		return d.RiskTier
	}
	return d.Tier
}

// TierForClass returns the ClassTier of a keeping row when the classifier
// vouches for a routine, riskless task. ok is false when the row keeps.
func (d DefaultProfile) TierForClass(routine, risky bool) (contract.Tier, bool) {
	if !d.Keeps() || !d.ClassTier.Valid() || !routine || risky {
		return contract.TierUnknown, false
	}
	return d.ClassTier, true
}

// harnessClaudeCode is the id the Claude Code adapter declares.
const harnessClaudeCode contract.HarnessID = "claude-code"

// defaultProfiles is the approved table N11 (2026-10-27), with the class step
// on general-purpose added 2026-10-28. The owner's own agents belong in the
// owner's policy.json, not here.
var defaultProfiles = []DefaultProfile{
	{Harness: harnessClaudeCode, AgentType: "Explore", Tier: contract.TierSmall, RiskTier: contract.TierMid},
	{Harness: harnessClaudeCode, AgentType: "claude-code-guide", Tier: contract.TierSmall, RiskTier: contract.TierMid},
	{Harness: harnessClaudeCode, AgentType: "statusline-setup", Tier: contract.TierSmall, RiskTier: contract.TierMid},
	{Harness: harnessClaudeCode, AgentType: "general-purpose", ClassTier: contract.TierMid},
	{Harness: harnessClaudeCode, AgentType: "Plan"},
}

// DefaultFor returns the default row for an agent type on a harness. The type
// match ignores case. An empty type, or one with no row, has no default.
func DefaultFor(harness contract.HarnessID, agentType string) (DefaultProfile, bool) {
	if agentType == "" {
		return DefaultProfile{}, false
	}
	for _, d := range defaultProfiles {
		if d.Harness == harness && strings.EqualFold(d.AgentType, agentType) {
			return d, true
		}
	}
	return DefaultProfile{}, false
}

// DefaultProfiles returns a copy of the table in declaration order.
func DefaultProfiles() []DefaultProfile {
	return append([]DefaultProfile(nil), defaultProfiles...)
}
