// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"slices"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
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
//     (a declared default tier of the agent, when the call names no model and
//     the tier is below the inherited one, sits here: it caps the hint, so a
//     hint never lands above it);
//  5. the classifier class: a decisive TRIVIAL or SIMPLE task with no risk
//     category moves down to small (classChoice), unless the policy sets
//     routing.by_class to false. It only moves down, never up (owner request
//     2026-11-01: route by task complexity as Ruflo does, applied here);
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
		// Profile names are lower case, and a harness reports its built-in types
		// with a capital (Explore, Plan): the match ignores case.
		name := contract.ProfileName(strings.ToLower(req.AgentType))
		if _, ok := in.Policy.ProfileNamed(name); ok {
			if c := profileChoice(in.Policy, req, name, contract.VerdictProfile, ""); c.verdict != contract.VerdictRefusedConfig {
				return c
			}
		}
	}
	if c, ok := ruleChoice(in); ok {
		return c
	}
	agent := agentDefault(in, requested)
	if t := req.Hint.Tier; t.Valid() && requested.Valid() && rank(t) < rank(requested) &&
		!(agent.Valid() && rank(agent) <= rank(t)) {
		return choice{verdict: contract.VerdictHintDown, tier: t}
	}
	if agent.Valid() {
		return choice{verdict: contract.VerdictAgentDefault, tier: agent}
	}
	if c, ok := classChoice(in, requested); ok {
		return c
	}
	return keep
}

// agentDefault is the declared tier of the spawned agent when it applies: the
// call named no model, the inherited tier is known, and the declared tier is
// below it. It only moves down; a pin, profile or rule has already won.
func agentDefault(in Input, requested contract.Tier) contract.Tier {
	t := in.AgentDefaultTier
	if !in.Policy.AgentDefaultRouting() || !in.Request.ModelInherited || !t.Valid() || !requested.Valid() || rank(t) >= rank(requested) {
		return contract.TierUnknown
	}
	return t
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
