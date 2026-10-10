// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import (
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routingv2/domain"
	"github.com/tiagovilasboas/downshift/internal/routingv2/extractor"
)

// MemoryKey builds the adapt-memory map key: classifier class + dominant feature,
// optionally prefixed with a stable repo identifier (never a filesystem path).
func MemoryKey(complexityClass, dominantFeature, repoID string) string {
	base := domain.MemoryShapeKey(complexityClass, dominantFeature)
	if base == "" {
		return ""
	}
	repoID = strings.TrimSpace(repoID)
	if repoID == "" {
		return base
	}
	return repoID + "/" + base
}

// MemoryKeyForPrompt derives the key from a prompt using ClassifyWithSemantic.
func MemoryKeyForPrompt(prompt, repoID string) string {
	fv := extractor.Extract(prompt)
	feature := domain.DominantFeature(fv)
	if feature == "" {
		return ""
	}
	cls := core.ClassifyWithSemantic(prompt)
	return MemoryKey(cls.Complexity.String(), feature, repoID)
}

// MemoryKeyForFeatures derives the key when only stored features exist (e.g. feedback sync).
// complexityClass must be the classifier label (TRIVIAL/SIMPLE/MEDIUM/COMPLEX).
func MemoryKeyForFeatures(complexityClass string, fv domain.FeatureVector, repoID string) string {
	feature := domain.DominantFeature(fv)
	return MemoryKey(complexityClass, feature, repoID)
}
