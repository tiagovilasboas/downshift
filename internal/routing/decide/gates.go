// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"slices"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/headroom"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// gatesPass reports whether every gate admits the call. Each gate asks
// headroom about one provider and one window with the gate's own limit as the
// threshold: the gate fails when that window is rate limited or at or past
// Below. A window with no fresh reading is unknown, and unknown passes.
func gatesPass(gates []policy.Gate, readings []contract.Reading, now time.Time) bool {
	for _, g := range gates {
		window := slices.DeleteFunc(slices.Clone(readings), func(r contract.Reading) bool {
			return r.Window != g.Window
		})
		for _, v := range headroom.Assess([]contract.HarnessID{g.Harness}, window, now, g.Below) {
			if v.Strain == headroom.StrainCritical {
				return false
			}
		}
	}
	return true
}

// relieveStrain applies the provider strain to a chosen tier (O3 and Fallback).
// When every provider of the set is critical, the requested model is kept: the
// note carries the warning (headroom ignored) and nothing is blocked. When only
// some are, a chosen profile whose provider is critical is replaced by its
// Fallback, provided the fallback is a valid profile of this harness at the
// same or a lower tier; otherwise the choice stands.
func relieveStrain(in Input, c choice) choice {
	verdicts := headroom.Assess(providers(in), in.Readings, in.Now, threshold(in))
	if headroom.AllCritical(verdicts) {
		return keep
	}
	p, ok := in.Policy.ProfileNamed(c.profile)
	if !ok || p.Fallback == "" || !critical(verdicts, p.Harness) {
		return c
	}
	fb := profileChoice(in.Policy, in.Request, p.Fallback, c.verdict, c.ruleID)
	if fb.verdict == contract.VerdictRefusedConfig || rank(fb.tier) > rank(c.tier) {
		return c
	}
	return fb
}

// providers is the set the strain is read over: the request's harness plus the
// harness of every declared profile.
func providers(in Input) []contract.HarnessID {
	set := []contract.HarnessID{in.Request.Harness}
	if in.Policy != nil {
		for _, p := range in.Policy.Profiles {
			set = append(set, p.Harness)
		}
	}
	return set
}

func threshold(in Input) float64 {
	if in.Policy != nil && in.Policy.CriticalThreshold > 0 {
		return in.Policy.CriticalThreshold
	}
	return headroom.DefaultCriticalThreshold
}

func critical(vs []headroom.Verdict, id contract.HarnessID) bool {
	for _, v := range vs {
		if v.Harness == id {
			return v.Strain == headroom.StrainCritical
		}
	}
	return false
}
