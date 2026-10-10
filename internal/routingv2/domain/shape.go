// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package domain

// DominantFeature returns the name of the strongest signal in the vector.
// An all-zero vector returns "" so unrelated no-signal tasks do not share a shape.
func DominantFeature(f FeatureVector) string {
	names := FeatureNames()
	vals := f.AsSlice()
	var (
		max   float64
		index = -1
	)
	for i, v := range vals {
		if v > max {
			max = v
			index = i
		}
	}
	if index < 0 || max <= 0 {
		return ""
	}
	return names[index]
}

// MemoryShapeKey scopes adapt memory to classifier complexity plus dominant feature.
// Empty dominant feature or complexity class yields "" (no learning).
func MemoryShapeKey(complexityClass, dominantFeature string) string {
	if dominantFeature == "" || complexityClass == "" || complexityClass == "UNKNOWN" {
		return ""
	}
	return complexityClass + ":" + dominantFeature
}
