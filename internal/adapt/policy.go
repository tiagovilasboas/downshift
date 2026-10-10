// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

// AdjustTier applies local tier memory to the baseline tier from classification.
// When memory is nil/empty or the shape is unknown, baseline is returned unchanged.
// A shape with hits only (no misses) still downshifts to the cheapest tier that hit;
// "frozen" in feedback ingestion stops new experiments, not tier memory.
func AdjustTier(baseline Tier, shape string, mem *Memory) Tier {
	if mem == nil || mem.empty() || shape == "" {
		return baseline
	}
	rec, ok := mem.Shapes[shape]
	if !ok {
		return baseline
	}
	hits := rec.hitSet()
	misses := rec.missSet()
	current := clampTier(baseline)

	if t, ok := cheapestHitBelow(current, hits); ok {
		return t
	}
	if hits[current] {
		return current
	}
	if misses[current] {
		if t, ok := cheapestHitAbove(current, hits); ok {
			return t
		}
	}
	return current
}

func cheapestHitBelow(current Tier, hits map[Tier]bool) (Tier, bool) {
	for t := TierSmall; t < current; t++ {
		if hits[t] {
			return t, true
		}
	}
	return 0, false
}

func cheapestHitAbove(current Tier, hits map[Tier]bool) (Tier, bool) {
	for t := current + 1; t <= TierFrontier; t++ {
		if hits[t] {
			return t, true
		}
	}
	return 0, false
}
