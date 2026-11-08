// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"errors"
	"fmt"

	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

// RefusalBudget: the orchestration already spent its token cap, so no wave started.
const RefusalBudget = "budget"

// orchestrationCap is the configured cap on the session's worker tokens, or 0
// (no cap) when none is wired. It is read on every call, so an edit applies to
// the next one.
func (d Deps) orchestrationCap() int64 {
	if d.BudgetTokens == nil {
		return 0
	}
	return d.BudgetTokens()
}

// startWave hands the specs to the queue as one wave under budget tokens (0 for
// none) and the orchestration cap. The queue owns the decision; this maps its
// refusals to codes and fixes. It returns the job ids and the wave number.
func (d Deps) startWave(specs []workers.Spec, budget int64, rec *delegateRecord) ([]string, int, error) {
	ids, wave, err := d.Work.DelegateWave(specs, workers.WaveLimits{Wave: budget, Orchestration: d.orchestrationCap()})
	var over workers.ErrOrchestrationBudget
	if errors.As(err, &over) {
		rec.code = RefusalBudget
		return nil, 0, fmt.Errorf("no worker started (%s): %w; work without workers or raise orchestration.budget_tokens or %s", RefusalBudget, err, policy.BudgetTokensEnv)
	}
	return ids, wave, err
}
