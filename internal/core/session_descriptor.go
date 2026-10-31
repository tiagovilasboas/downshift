// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"math"
	"regexp"
)

// Session descriptor validation and normalization. These helpers vet and clean
// a single reported descriptor field (tier, price, effort) before the resolver
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

// sanitizePrice keeps a finite, non-negative list price, else 0 (unknown).
// Price never drives routing (owner decision 2026-10-31); this only keeps a
// stored value safe for a reader and never rejects a descriptor.
func sanitizePrice(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return 0
	}
	return value
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
