// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "regexp"

// Session descriptor validation and normalization. These helpers vet and clean
// a single reported descriptor field (tier, effort) before the resolver
// stores it. They change for a different reason than model selection, so they
// live apart from session_resolver.go.

func sessionTier(value string) (Tier, bool) {
	switch value {
	case "small":
		return TierSmall, true
	case "mid":
		return TierMid, true
	case "frontier":
		return TierFrontier, true
	}
	return TierSmall, false
}

var sessionEffortToken = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func validSessionEfforts(efforts []string) bool {
	for _, effort := range efforts {
		if !sessionEffortToken.MatchString(effort) {
			return false
		}
	}
	return true
}
