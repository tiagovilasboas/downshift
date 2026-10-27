// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// combine merges the project layer over the user layer. A setting the project
// file sets wins; otherwise the user's value is used; otherwise the default.
// Profiles and rules the project declares replace the user's with the same
// name or id, and the project's come first, so a project rule is tried before
// a user rule.
func combine(project, user layer) *Policy {
	p := &Policy{
		CriticalThreshold: DefaultCriticalThreshold,
		BothCritical:      DefaultBothCritical,
		QuotaTTL:          DefaultQuotaTTL,
	}
	if v := firstSet(project.threshold, user.threshold); v != nil {
		p.CriticalThreshold = *v
	}
	if v := firstSet(project.both, user.both); v != nil {
		p.BothCritical = *v
	}
	if v := firstSet(project.ttl, user.ttl); v != nil {
		p.QuotaTTL = *v
	}
	p.Profiles = overlay(project.profiles, user.profiles, func(x Profile) string { return string(x.Name) })
	p.Rules = overlay(project.rules, user.rules, func(x Rule) string { return string(x.ID) })
	return p
}

func firstSet[T any](values ...*T) *T {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

// overlay returns the project's entries, then the user's entries whose key the
// project does not declare. Within one layer keys are already unique.
func overlay[T any](project, user []T, key func(T) string) []T {
	taken := make(map[string]bool, len(project))
	out := make([]T, 0, len(project)+len(user))
	for _, x := range project {
		taken[key(x)] = true
		out = append(out, x)
	}
	for _, x := range user {
		if !taken[key(x)] {
			out = append(out, x)
		}
	}
	return out
}

// checkRefs checks what spans both files, on the merged policy. A rule's
// profile must exist. A fallback must exist, must not be the profile itself,
// must not be a higher tier (a fallback never widens), and must not declare a
// fallback of its own (no chains).
func (p *Policy) checkRefs() error {
	for _, pr := range p.Profiles {
		if pr.Fallback == "" {
			continue
		}
		field := fmt.Sprintf("routing.profiles[%s].fallback", pr.Name)
		target, ok := p.ProfileNamed(pr.Fallback)
		switch {
		case !ok:
			return unknownProfile(pr.source, field, pr.Fallback)
		case pr.Fallback == pr.Name:
			return refuse(pr.source, field, "a profile cannot fall back to itself")
		case tierRank(target.Tier) > tierRank(pr.Tier):
			return refuse(pr.source, field, "fallback %q is tier %s, above %s: a fallback never widens", target.Name, target.Tier, pr.Tier)
		case target.Fallback != "":
			return refuse(pr.source, field, "fallback %q declares its own fallback; chains are not supported", target.Name)
		}
	}
	for _, r := range p.Rules {
		if r.Profile == "" {
			continue
		}
		if _, ok := p.ProfileNamed(r.Profile); !ok {
			return unknownProfile(r.source, fmt.Sprintf("routing.rules[%s].profile", r.ID), r.Profile)
		}
	}
	return nil
}

func unknownProfile(source, field string, name contract.ProfileName) error {
	return &ConfigError{Source: source, Field: field, Problem: fmt.Sprintf("unknown profile %q", name), Reason: contract.ReasonUnknownProfile}
}

// tierRank orders tiers from cheapest to strongest; an unknown tier ranks below
// every known one.
func tierRank(t contract.Tier) int {
	ct, ok := t.Core()
	if !ok {
		return -1
	}
	return int(ct)
}
