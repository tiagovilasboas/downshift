// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "github.com/tiagovilasboas/downshift/internal/classify"

// Complexity and Classification stay addressable as core types so route
// policy and adapters do not import the classifier. The values are the
// classifier's types; route policy maps them with TierOf.
type (
	Complexity     = classify.Complexity
	Classification = classify.Classification
)

const (
	Trivial = classify.Trivial
	Simple  = classify.Simple
	Medium  = classify.Medium
	Complex = classify.Complex
)

// Classify scores a prompt with the regex classifier. The semantic boost
// policy lives in internal/fuse; ClassifyWithSemantic is the wrapper.
func Classify(prompt string) Classification { return classify.Classify(prompt) }

// RiskCategories and RiskCategoryNames are the classifier's risk table,
// re-exported for policy floors and project policy files.
func RiskCategories(prompt string) []string { return classify.RiskCategories(prompt) }

func RiskCategoryNames() map[string]bool { return classify.RiskCategoryNames() }

func applyRiskFloor(prompt string, c Classification) Classification {
	return classify.ApplyRiskFloor(prompt, c)
}
