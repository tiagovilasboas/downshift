// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

// featureNames mirrors domain.FeatureNames order without importing domain.
var featureNames = []string{
	"mechanical",
	"coding",
	"debugging",
	"refactoring",
	"architecture",
	"migration",
	"security",
	"concurrency",
	"planning",
	"tool_use",
	"ambiguity",
	"cross_module",
	"context_size",
}

// ShapeFromValues returns the dominant feature name for a 13-element vector.
// All-zero vectors return "" so no-signal tasks do not share a shape.
func ShapeFromValues(values [13]float64) string {
	var (
		max   float64
		index = -1
	)
	for i, v := range values {
		if v > max {
			max = v
			index = i
		}
	}
	if index < 0 || max <= 0 {
		return ""
	}
	return featureNames[index]
}
