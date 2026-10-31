// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package policy declares the routing policy: named profiles, ordered rules
// with quota gates, and the routing settings. It loads and validates the
// declaration; it decides nothing. Matching a request to a model is
// routing/decide's job, and reading quota is routing/headroom's job.
//
// Config errors are refused here, at load time (design section 2): a bad file
// yields an error and no policy. A missing file is not an error; it is an
// empty policy, so the requested model is kept (doubt keeps).
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package policy

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// Defaults applied when a file leaves a setting out.
const (
	DefaultCriticalThreshold = 80.0
	DefaultQuotaTTL          = 5 * time.Minute
	DefaultBothCritical      = BothCriticalAsk
)

// BothCritical is the declared policy for when both providers are past the
// critical threshold. Only the enum and its validation live here; the decider
// applies it in a later card, and a hook must never block on it (fail open).
type BothCritical string

const (
	BothCriticalAsk         BothCritical = "ask"
	BothCriticalRunCheapest BothCritical = "run-cheapest"
	BothCriticalBlock       BothCritical = "block"
)

// BothCriticals lists the declared values in a stable order.
func BothCriticals() []BothCritical {
	return []BothCritical{BothCriticalAsk, BothCriticalRunCheapest, BothCriticalBlock}
}

// Valid reports whether b is a declared value.
func (b BothCritical) Valid() bool { return contains(BothCriticals(), b) }

// Policy is the merged and validated routing policy. Profiles and Rules keep
// their declaration order: project entries first, then user entries the
// project does not replace. Rules are first-match-wins.
type Policy struct {
	CriticalThreshold float64 // percent, 0..100
	BothCritical      BothCritical
	QuotaTTL          time.Duration // age after which a quota reading counts as missing
	Mode              Mode
	Profiles          []Profile
	Rules             []Rule
}

// Profile is a named tier on one harness. It is the only source of what it
// defines (REA section 2): a request that names it cannot also override its
// tier or model (see CheckProfile). Fallback names one other profile, used
// only at the same or a lower tier, with no chain. Description is the user's own
// words for what the profile is for. It is optional and routing ignores it;
// `policy apply` copies it into the agent file it creates, because a harness
// needs one to load an agent.
type Profile struct {
	Name        contract.ProfileName
	Tier        contract.Tier
	Harness     contract.HarnessID
	Fallback    contract.ProfileName
	Description string
	source      string
}

// Rule maps the kinds, efforts and classifier classes it admits, plus quota
// gates, to a profile or a tier. Exactly one of Profile and Tier is set.
type Rule struct {
	ID      contract.RuleID
	Enabled bool
	Kinds   []contract.Kind        // empty admits any kind
	Efforts []contract.EffortClass // empty admits any effort
	// Classes is the classifier class gate (owner decision O8). Empty admits
	// any class, so a class-free rule behaves exactly as before. A non-empty
	// list admits a call only when all three hold: the classifier's class is in
	// the set, the classification is confident, and the task carries no risk
	// category. This is the one way to send a TRIVIAL/SIMPLE task down to a
	// smaller tier, since the project floor only ever raises the tier.
	Classes []contract.Complexity
	Gates   []Gate // declared, not evaluated yet: all must pass; a gate with no reading passes
	Profile contract.ProfileName
	Tier    contract.Tier
	source  string
}

// MatchContext is what a rule is matched against: the declared kind and effort
// (from the hint) and the classifier's reading of the task. Class is the class
// string ("TRIVIAL".."COMPLEX"); Confident and HasRisk come straight from the
// classifier. A rule with no Classes ignores all three class fields.
type MatchContext struct {
	Kind      contract.Kind
	Effort    contract.EffortClass
	Class     contract.Complexity
	Confident bool
	HasRisk   bool
}

// Gate passes while the named window of the harness is below Below percent.
type Gate struct {
	Harness contract.HarnessID
	Window  contract.Window
	Below   float64
}

// ProfileNamed returns the profile with the given name. Nil-safe.
func (p *Policy) ProfileNamed(name contract.ProfileName) (Profile, bool) {
	if p == nil {
		return Profile{}, false
	}
	for _, pr := range p.Profiles {
		if pr.Name == name {
			return pr, true
		}
	}
	return Profile{}, false
}

// Match returns the first enabled rule that admits the context, in declaration
// order. It ignores Gates: no code evaluates them here, and headroom.Assess
// returns only the strain of each provider. A rule with a non-empty Kinds or
// Efforts list admits only a call that declares a value in it, so a call with
// no declared kind or effort matches only rules open on that dimension. A rule
// with a non-empty Classes list admits the call only when admitsClass holds.
// Nil-safe.
func (p *Policy) Match(ctx MatchContext) (Rule, bool) {
	if p == nil {
		return Rule{}, false
	}
	for _, r := range p.Rules {
		if !r.Enabled {
			continue
		}
		if len(r.Kinds) > 0 && !contains(r.Kinds, ctx.Kind) {
			continue
		}
		if len(r.Efforts) > 0 && !contains(r.Efforts, ctx.Effort) {
			continue
		}
		if !admitsClass(r.Classes, ctx) {
			continue
		}
		return r, true
	}
	return Rule{}, false
}

// admitsClass reports whether a rule's class gate admits the context. An empty
// gate admits anything (the pre-O8 behaviour). A non-empty gate admits only a
// confident, risk-free classification whose class is in the set: doubt keeps,
// and any risk category keeps, so a class rule can never send a risky or
// uncertain task down.
func admitsClass(classes []contract.Complexity, ctx MatchContext) bool {
	if len(classes) == 0 {
		return true
	}
	if !ctx.Confident || ctx.HasRisk {
		return false
	}
	return contains(classes, ctx.Class)
}

func contains[T comparable](set []T, v T) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}
