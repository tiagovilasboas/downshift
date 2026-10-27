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
	Profiles          []Profile
	Rules             []Rule
}

// Profile is a named tier on one harness. It is the only source of what it
// defines (REA section 2): a request that names it cannot also override its
// tier or model (see CheckProfile). Fallback names one other profile, used
// only at the same or a lower tier, with no chain.
type Profile struct {
	Name     contract.ProfileName
	Tier     contract.Tier
	Harness  contract.HarnessID
	Fallback contract.ProfileName
	source   string
}

// Rule maps the kinds and efforts it admits, plus quota gates, to a profile or
// a tier. Exactly one of Profile and Tier is set.
type Rule struct {
	ID      contract.RuleID
	Enabled bool
	Kinds   []contract.Kind        // empty admits any kind
	Efforts []contract.EffortClass // empty admits any effort
	Gates   []Gate                 // all must pass; a gate with no reading passes
	Profile contract.ProfileName
	Tier    contract.Tier
	source  string
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

// Match returns the first enabled rule that admits the kind and effort, in
// declaration order. Gates are not evaluated here; headroom does that. A rule
// with a non-empty list admits only a call that declares a value in it, so a
// call with no declared kind or effort matches only rules open on that
// dimension. Nil-safe.
func (p *Policy) Match(kind contract.Kind, effort contract.EffortClass) (Rule, bool) {
	if p == nil {
		return Rule{}, false
	}
	for _, r := range p.Rules {
		if !r.Enabled {
			continue
		}
		if len(r.Kinds) > 0 && !contains(r.Kinds, kind) {
			continue
		}
		if len(r.Efforts) > 0 && !contains(r.Efforts, effort) {
			continue
		}
		return r, true
	}
	return Rule{}, false
}

func contains[T comparable](set []T, v T) bool {
	for _, s := range set {
		if s == v {
			return true
		}
	}
	return false
}
