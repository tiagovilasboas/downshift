// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

// CheckProfile refuses a request that names a profile and also sets what the
// profile defines. A profile is the only source of its tier and harness (REA
// section 2: "a role is the only source"), so a pinned model or a hint tier
// would override it and the request is a config error. A request with no
// profile passes. The decider logs the refusal and keeps the spawn unchanged.
func (p *Policy) CheckProfile(req contract.Request) error {
	if req.Profile == "" {
		return nil
	}
	pr, ok := p.ProfileNamed(req.Profile)
	if !ok {
		return &ConfigError{Field: "profile", Problem: fmt.Sprintf("unknown profile %q", req.Profile), Reason: contract.ReasonUnknownProfile}
	}
	switch {
	case req.Pinned:
		return overrides(pr, "a pinned model would override its tier and harness")
	case req.Hint.Tier != contract.TierUnknown:
		return overrides(pr, "a hint tier would override its tier")
	case req.Harness != "" && req.Harness != pr.Harness:
		return &ConfigError{Field: "profile", Problem: fmt.Sprintf("profile %q resolves on %s, not on %s", pr.Name, pr.Harness, req.Harness), Reason: contract.ReasonForeignID}
	}
	return nil
}

func overrides(pr Profile, why string) error {
	return &ConfigError{Field: "profile", Problem: fmt.Sprintf("profile %q is the only source of what it defines: %s", pr.Name, why), Reason: contract.ReasonProfileOverride}
}
