// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"math"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// layer is one validated file. A setting the file leaves out stays nil, so the
// merge can fall back to the other file and then to the default.
type layer struct {
	source    string
	threshold *float64
	both      *BothCritical
	ttl       *time.Duration
	mode      *Mode
	profiles  []Profile
	rules     []Rule
}

// harnessNames is the set of harness ids the policy may name, read from the
// harness data in core (layering rule 3). It is only read, never written.
var harnessNames = func() map[contract.HarnessID]bool {
	names := make(map[contract.HarnessID]bool)
	for _, nc := range core.AllHarnessCaps() {
		names[contract.HarnessID(nc.Harness)] = true
	}
	return names
}()

// validateSection turns the decoded routing section of one file into a layer.
// The first problem found is returned; checks run in file order, so the same
// file always reports the same problem.
func validateSection(source string, s section) (layer, error) {
	l := layer{source: source}
	if s.CriticalThreshold != nil {
		if err := checkPercent(source, "routing.critical_threshold", *s.CriticalThreshold); err != nil {
			return layer{}, err
		}
		v := *s.CriticalThreshold
		l.threshold = &v
	}
	if s.BothCritical != nil {
		b := BothCritical(*s.BothCritical)
		if !b.Valid() {
			return layer{}, refuse(source, "routing.both_critical", "%q is not ask, run-cheapest or block", *s.BothCritical)
		}
		l.both = &b
	}
	if s.QuotaTTLSeconds != nil {
		secs := *s.QuotaTTLSeconds
		switch {
		case secs <= 0:
			return layer{}, refuse(source, "routing.quota_ttl_seconds", "must be positive, got %d", secs)
		case secs > math.MaxInt64/int64(time.Second):
			return layer{}, refuse(source, "routing.quota_ttl_seconds", "too large, got %d", secs)
		}
		ttl := time.Duration(secs) * time.Second
		l.ttl = &ttl
	}
	var err error
	if l.profiles, err = validateProfiles(source, s.Profiles); err != nil {
		return layer{}, err
	}
	if l.rules, err = validateRules(source, s.Rules); err != nil {
		return layer{}, err
	}
	return l, nil
}

func checkPercent(source, field string, v float64) error {
	if v >= 0 && v <= 100 {
		return nil
	}
	return refuse(source, field, "must be between 0 and 100, got %g", v)
}

func knownHarness(h contract.HarnessID) bool { return harnessNames[h] }

func validateProfiles(source string, docs []profileDoc) ([]Profile, error) {
	seen := make(map[contract.ProfileName]bool, len(docs))
	out := make([]Profile, 0, len(docs))
	for i, d := range docs {
		field := fmt.Sprintf("routing.profiles[%d]", i)
		name := contract.ProfileName(d.Name)
		if !name.Valid() {
			return nil, refuse(source, field+".name", "%q is not a valid name (lowercase letters, digits, dot, underscore or dash; up to 64)", d.Name)
		}
		if seen[name] {
			return nil, refuse(source, field+".name", "duplicate profile %q", d.Name)
		}
		seen[name] = true
		tier := contract.Tier(d.Tier)
		if !tier.Valid() {
			return nil, refuse(source, field+".tier", "%q is not small, mid or frontier", d.Tier)
		}
		harness := contract.HarnessID(d.Harness)
		if !knownHarness(harness) {
			return nil, refuse(source, field+".harness", "unknown harness %q", d.Harness)
		}
		fallback := contract.ProfileName(d.Fallback)
		if d.Fallback != "" && !fallback.Valid() {
			return nil, refuse(source, field+".fallback", "%q is not a valid profile name", d.Fallback)
		}
		out = append(out, Profile{Name: name, Tier: tier, Harness: harness, Fallback: fallback, Description: d.Description, source: source})
	}
	return out, nil
}

func validateRules(source string, docs []ruleDoc) ([]Rule, error) {
	seen := make(map[contract.RuleID]bool, len(docs))
	out := make([]Rule, 0, len(docs))
	for i, d := range docs {
		field := fmt.Sprintf("routing.rules[%d]", i)
		id := contract.RuleID(d.ID)
		if !id.Valid() {
			return nil, refuse(source, field+".id", "%q is not a valid id (lowercase letters, digits, dot, underscore or dash; up to 64)", d.ID)
		}
		if seen[id] {
			return nil, refuse(source, field+".id", "duplicate rule %q", d.ID)
		}
		seen[id] = true
		r := Rule{ID: id, Enabled: true, source: source}
		if d.Enabled != nil {
			r.Enabled = *d.Enabled
		}
		var err error
		if r.Kinds, err = validateKinds(source, field, d.Kinds); err != nil {
			return nil, err
		}
		if r.Efforts, err = validateEfforts(source, field, d.Efforts); err != nil {
			return nil, err
		}
		if r.Gates, err = validateGates(source, field, d.Gates); err != nil {
			return nil, err
		}
		if err := setTarget(source, field, &r, d.Profile, d.Tier); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func validateKinds(source, field string, values []string) ([]contract.Kind, error) {
	var out []contract.Kind
	for j, v := range values {
		k := contract.Kind(v)
		if !k.Valid() {
			return nil, refuse(source, fmt.Sprintf("%s.kinds[%d]", field, j), "unknown kind %q", v)
		}
		out = append(out, k)
	}
	return out, nil
}

func validateEfforts(source, field string, values []string) ([]contract.EffortClass, error) {
	var out []contract.EffortClass
	for j, v := range values {
		e := contract.EffortClass(v)
		if !e.Valid() {
			return nil, refuse(source, fmt.Sprintf("%s.efforts[%d]", field, j), "unknown effort class %q", v)
		}
		out = append(out, e)
	}
	return out, nil
}

func validateGates(source, field string, docs []gateDoc) ([]Gate, error) {
	var out []Gate
	for j, d := range docs {
		gf := fmt.Sprintf("%s.gates[%d]", field, j)
		h := contract.HarnessID(d.Harness)
		if !knownHarness(h) {
			return nil, refuse(source, gf+".harness", "unknown harness %q", d.Harness)
		}
		w := contract.Window(d.Window)
		if !w.Valid() {
			return nil, refuse(source, gf+".window", "%q is not short, week or model", d.Window)
		}
		if d.Below == nil {
			return nil, refuse(source, gf+".below", "required: the percentage the window must stay below")
		}
		if err := checkPercent(source, gf+".below", *d.Below); err != nil {
			return nil, err
		}
		out = append(out, Gate{Harness: h, Window: w, Below: *d.Below})
	}
	return out, nil
}

// setTarget gives a rule exactly one target. A profile is the only source of
// what it defines, so a rule that names a profile and a tier is refused.
func setTarget(source, field string, r *Rule, profile, tier string) error {
	switch {
	case profile != "" && tier != "":
		return refuse(source, field, "set profile or tier, not both: a profile is the only source of its tier")
	case profile == "" && tier == "":
		return refuse(source, field, "set one of profile or tier")
	case profile != "":
		name := contract.ProfileName(profile)
		if !name.Valid() {
			return refuse(source, field+".profile", "%q is not a valid profile name", profile)
		}
		r.Profile = name
	default:
		t := contract.Tier(tier)
		if !t.Valid() {
			return refuse(source, field+".tier", "%q is not small, mid or frontier", tier)
		}
		r.Tier = t
	}
	return nil
}
