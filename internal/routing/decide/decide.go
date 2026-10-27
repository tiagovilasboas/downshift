// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package decide builds the routing v2 decision of one spawn: it walks the
// precedence of docs/design/routing-v2.md section 2 (resolve.go), asks the
// guard whether the move is allowed, and fills a core.Decision plus the
// routing note. It is pure: the policy, the quota readings, the clock, the
// risk categories and the catalog are all inputs, and nothing is read or
// written. It records tiers, ids and enums only, never task text or paths.
//
// Rules the code keeps:
//   - doubt keeps: no catalog, no policy, an unknown requested model or any
//     panic leaves the requested model in place (verdict kept, or refused_config
//     for a config error, with the reason);
//   - the guard has the last word: a move happens only when guard.Check allows
//     it, so risk never goes down, a target above the request is refused,
//     explicit_only and foreign ids are never written;
//   - price is not used: the catalog supplies ids and tiers, never dollars, so
//     Decision.Savings stays zero;
//   - the decision is per spawn: it reads no session list.
//
// A profile's Fallback and the policy's BothCritical are not applied here:
// strain is advice for gates and the note, and never reroutes.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package decide

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/guard"
	"github.com/tiagovilasboas/downshift/internal/routing/headroom"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// Catalog is the harness model catalog the decision reads. core.Resolver with
// the exact-id extension (catalog.Catalog) satisfies it.
type Catalog interface {
	guard.Catalog
	ModelFor(harness string, tier core.Tier) core.Model
	LookupByID(harness, modelID string) (core.Model, bool)
}

// Input is one spawn to decide. Risk holds the risk categories of the task
// text, scanned by the caller; the text itself never reaches this package.
type Input struct {
	Request  contract.Request
	Policy   *policy.Policy // nil is an empty policy
	Readings []contract.Reading
	Now      time.Time
	Risk     []string
	Catalog  Catalog
}

// Decide returns the decision and its routing note. The note's Channel is left
// for the caller, which knows how the decision is applied.
func Decide(in Input) (d core.Decision, note contract.Note) {
	defer func() {
		if recover() != nil {
			d, note = failOpen(in)
		}
	}()
	if in.Catalog == nil {
		return kept(in, core.Model{}, contract.TierUnknown, choice{verdict: contract.VerdictKept})
	}
	req := in.Request
	current, known := in.Catalog.LookupByID(string(req.Harness), req.RequestedModel)
	if req.RequestedModel == "" {
		known = false
	}
	requested := contract.TierUnknown
	if known {
		requested = contract.TierOf(current.Tier)
	}

	c := resolve(in, requested)
	if !c.tier.Valid() {
		return kept(in, current, requested, c)
	}
	target := targetModel(in, c.tier)
	if target.ID == "" && rank(c.tier) < rank(requested) {
		c.verdict, c.reason = contract.VerdictHeldUnknown, contract.ReasonNoCandidate
		return kept(in, current, requested, c)
	}
	res := guard.Check(in.Catalog, guard.Input{
		Harness: req.Harness, Risk: in.Risk, Confident: known,
		Requested: requested, Target: c.tier, TargetID: target.ID,
	})
	if res.Allowed {
		return moved(in, current, target, requested, c)
	}
	switch res.Reason {
	case contract.ReasonNone:
		c.verdict = contract.VerdictKept
	case contract.ReasonRiskFloor:
		c.verdict, c.reason = contract.VerdictGuardFloor, res.Reason
	default:
		c.verdict, c.reason = contract.VerdictHeldUnknown, res.Reason
	}
	return kept(in, current, requested, c)
}

func targetModel(in Input, tier contract.Tier) core.Model {
	t, ok := tier.Core()
	if !ok {
		return core.Model{}
	}
	return in.Catalog.ModelFor(string(in.Request.Harness), t)
}

// moved is a downshift the guard allowed.
func moved(in Input, current, target core.Model, requested contract.Tier, c choice) (core.Decision, contract.Note) {
	d := base(in, current)
	d.Tier, d.Model = target.Tier, target
	d.Effort = core.EffortFor(target.Tier)
	d.Verdict, d.SafeVerdict = core.VerdictDownshift, core.VerdictDownshift
	d.Confident = true
	return d, note(in, c, requested, contract.TierOf(target.Tier), target)
}

// kept leaves the requested model in place. c carries the verdict and the
// reason for the note; a refused_config or held reason is also on the Decision.
func kept(in Input, current core.Model, requested contract.Tier, c choice) (core.Decision, contract.Note) {
	d := base(in, current)
	d.Reason = string(c.reason)
	d.Verdict, d.SafeVerdict = core.VerdictUnknown, core.VerdictUnknown
	if t, ok := requested.Core(); ok {
		d.Tier, d.Model = t, current
		d.Effort = core.EffortFor(t)
		d.Verdict, d.SafeVerdict = core.VerdictOK, core.VerdictOK
		d.Confident = true
	}
	return d, note(in, c, requested, requested, current)
}

// failOpen is the answer after a panic: nothing moves, nothing is claimed.
func failOpen(in Input) (core.Decision, contract.Note) {
	d := base(in, core.Model{})
	d.Reason = string(contract.ReasonFailOpen)
	d.Verdict, d.SafeVerdict = core.VerdictUnknown, core.VerdictUnknown
	n := contract.Note{Verdict: contract.VerdictRefusedConfig, Reason: contract.ReasonFailOpen, Harness: in.Request.Harness}
	return d, n
}

func base(in Input, current core.Model) core.Decision {
	return core.Decision{
		Harness: string(in.Request.Harness),
		Risk:    in.Risk,
		Checked: true,
		Baseline: core.Baseline{
			CurrentModel: current,
			RequestedID:  in.Request.RequestedModel,
			Pinned:       in.Request.Pinned,
		},
	}
}

// note builds the routing note: the verdict, tiers and the headroom of the
// harness the decision landed on. A fresh reading below the threshold reads as
// chosen, one at or past it as ignored (the decision never reroutes on it).
func note(in Input, c choice, from, to contract.Tier, landed core.Model) contract.Note {
	n := contract.Note{
		Verdict: c.verdict, Reason: c.reason, RuleID: c.ruleID, Profile: c.profile,
		TierFrom: from, TierTo: to, Harness: in.Request.Harness,
	}
	threshold := float64(headroom.DefaultCriticalThreshold)
	if in.Policy != nil && in.Policy.CriticalThreshold > 0 {
		threshold = in.Policy.CriticalThreshold
	}
	provider := in.Request.Harness
	if landed.Harness != "" {
		provider = contract.HarnessID(landed.Harness)
	}
	for _, v := range headroom.Assess([]contract.HarnessID{provider}, in.Readings, in.Now, threshold) {
		n.HeadroomWindow, n.HeadroomUsedPct, n.RateLimited = v.Window, v.UsedPct, v.RateLimited
		switch v.Strain {
		case headroom.StrainOK:
			n.Headroom = contract.HeadroomChosen
		case headroom.StrainCritical:
			n.Headroom = contract.HeadroomIgnored
		}
	}
	return n
}
