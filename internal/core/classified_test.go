// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import "github.com/tiagovilasboas/downshift/internal/core"

// classifiedRoute is what the classifier alone says about a prompt, shaped as
// a Decision: complexity, tier, confidence, risk and the classifier guards. No
// routing policy runs: the model, verdict and corrections stay empty. The
// classifier tests (corpus, stability, risk, language, semantic) use it to
// keep measuring classify and fuse, which outcome and benchmark still read.
// The signature mirrors core.Route so those tests changed one call only; the
// current model and the resolver are ignored.
func classifiedRoute(prompt, harness, _ string, _ ...core.Resolver) core.Decision {
	cls := core.ClassifyWithSemantic(prompt)
	tier := core.TierOf(cls.Complexity)
	return core.Decision{
		Complexity:          cls.Complexity,
		Tier:                tier,
		Effort:              core.EffortFor(tier),
		Harness:             harness,
		Confident:           cls.Confident,
		Risk:                core.RiskCategories(prompt),
		NoSignal:            cls.NoSignal,
		UnsupportedLanguage: cls.UnsupportedLanguage,
		Guards: core.Guards{
			RiskFloor:    cls.RiskFloor,
			RiskAbstain:  cls.RiskAbstain,
			SemanticLift: cls.SemanticLift,
			TaskFocus:    cls.TaskFocus,
		},
	}
}
