// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"slices"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// choice is what the precedence ladder picked, before the guard sees it. A
// verdict of kept or refused_config means no tier was chosen and the requested
// model stays.
type choice struct {
	verdict contract.Verdict
	tier    contract.Tier
	ruleID  contract.RuleID
	profile contract.ProfileName
	reason  contract.Reason
}

var keep = choice{verdict: contract.VerdictKept}

func refused(reason contract.Reason) choice {
	return choice{verdict: contract.VerdictRefusedConfig, reason: reason}
}

// resolve walks the precedence of design section 2, first match wins:
//  1. a pinned model is kept; a pin plus a named profile is a config error;
//  2. a profile, named by the request or matching its agent type;
//  3. the first enabled rule whose kind, effort and quota gates admit it;
//  4. the hint's tier, only when it is below the requested tier;
//  5. the default profile table shipped with the binary, by agent type;
//  6. otherwise the requested model is kept.
//
// requested is the tier of the requested model; it is unknown (empty) when the
// catalog does not know the model, and only the hint step reads it.
func resolve(in Input, requested contract.Tier) choice {
	req := in.Request
	if req.Pinned {
		if req.Profile != "" {
			return refused(contract.ReasonProfileOverride)
		}
		return keep
	}
	if req.Profile != "" {
		return profileChoice(in.Policy, req, req.Profile, contract.VerdictProfile, "")
	}
	if req.AgentType != "" {
		name := contract.ProfileName(req.AgentType)
		if _, ok := in.Policy.ProfileNamed(name); ok {
			if c := profileChoice(in.Policy, req, name, contract.VerdictProfile, ""); c.verdict != contract.VerdictRefusedConfig {
				return c
			}
		}
	}
	if c, ok := ruleChoice(in); ok {
		return c
	}
	if t := req.Hint.Tier; t.Valid() && requested.Valid() && rank(t) < rank(requested) {
		return choice{verdict: contract.VerdictHintDown, tier: t}
	}
	// A declared hint tier, even one equal to the request, is the parent's
	// word: the default table does not override it.
	if req.Hint.Tier.Valid() {
		return keep
	}
	return defaultChoice(in, requested)
}

// defaultChoice reads the default profile table for the agent type. It keeps
// when no row applies, when the row keeps (standard mode), when the policy
// declares a profile of that name (a declared profile is never replaced by the
// table, even one refused for another harness), or when the row would raise
// the tier: a default only moves down, and an upshift is the guard's business,
// not the table's. Economy mode (economy.go) changes the tier a row asks for,
// never the precedence above.
func defaultChoice(in Input, requested contract.Tier) choice {
	req := in.Request
	if _, declared := in.Policy.ProfileNamed(contract.ProfileName(req.AgentType)); declared {
		return keep
	}
	d, ok := policy.DefaultFor(req.Harness, req.AgentType)
	if !ok {
		return keep
	}
	tier, ok := rowTier(d, len(in.Risk) > 0, in.routine(), requested, in.Policy.EconomyMode())
	if !ok || rank(tier) > rank(requested) {
		return keep
	}
	return choice{verdict: contract.VerdictDefault, tier: tier}
}

// profileChoice reads the tier of a profile of this harness. A profile that
// is not declared, or declared for another harness, is a config error.
func profileChoice(pol *policy.Policy, req contract.Request, name contract.ProfileName,
	verdict contract.Verdict, rule contract.RuleID) choice {
	p, ok := pol.ProfileNamed(name)
	if !ok || p.Harness != req.Harness {
		return refused(contract.ReasonUnknownProfile)
	}
	return choice{verdict: verdict, tier: p.Tier, ruleID: rule, profile: name}
}

// ruleChoice returns the first matching rule. Rules whose quota gates fail are
// dropped before matching, so the next rule in declaration order is tried.
func ruleChoice(in Input) (choice, bool) {
	if in.Policy == nil {
		return choice{}, false
	}
	open := *in.Policy
	open.Rules = slices.DeleteFunc(slices.Clone(in.Policy.Rules), func(r policy.Rule) bool {
		return !gatesPass(r.Gates, in.Readings, in.Now)
	})
	rule, ok := open.Match(in.Request.Hint.Kind, in.Request.Hint.Effort)
	if !ok {
		return choice{}, false
	}
	if rule.Profile != "" {
		return profileChoice(in.Policy, in.Request, rule.Profile, contract.VerdictRule, rule.ID), true
	}
	return choice{verdict: contract.VerdictRule, tier: rule.Tier, ruleID: rule.ID}, true
}

// rank orders tiers from cheapest (0); an unknown tier is -1.
func rank(t contract.Tier) int { return slices.Index(contract.Tiers(), t) }
