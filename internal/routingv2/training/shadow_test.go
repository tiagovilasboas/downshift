// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package training

import (
	"errors"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/domain"
	"github.com/tiagovilasboas/harness-downshift/internal/routingv2/shadow"
)

type fixedShadow struct {
	probs domain.TierProbabilities
	err   error
}

func (p fixedShadow) Predict(shadow.Input) (domain.TierProbabilities, error) { return p.probs, p.err }

func TestShadowReportUsesLabelsNotBinaryOutcomes(t *testing.T) {
	id := "softmax-sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	unsafe := shadow.Evaluate(fixedShadow{probs: domain.TierProbabilities{Small: .95, Mid: .04, Frontier: .01}}, id,
		shadow.Input{Features: domain.FeatureVector{Security: .9}})
	frontier := core.TierFrontier
	errorObservation := shadow.Evaluate(fixedShadow{err: errors.New("private")}, id, shadow.Input{})
	events := []Event{
		{SelectedTier: core.TierMid, Shadow: &unsafe, Outcome: &Outcome{Failed: true, RequiredTier: &frontier}},
		{SelectedTier: core.TierMid, Shadow: &unsafe, Outcome: &Outcome{Success: true}},
		{SelectedTier: core.TierMid, Shadow: &unsafe},
		{SelectedTier: core.TierMid, Shadow: &errorObservation},
		{Shadow: &shadow.Observation{ErrorCode: "load_error"}},
		{Shadow: &shadow.Observation{ModelID: "untrusted metadata"}},
		{}, // Pre-shadow events remain readable and are excluded.
	}
	r := SummarizeShadow(events)
	if r.LoadErrors != 1 || r.Invalid != 1 || len(r.Models) != 1 {
		t.Fatalf("report=%+v", r)
	}
	m := r.Models[0]
	if m.Observed != 3 || m.Errors != 1 || m.Disagreements != 3 || m.Upshifts != 3 || m.Downshifts != 0 ||
		m.ReviewedOutcomes != 2 || m.ProductionFailures != 1 || m.Labeled != 1 ||
		m.RawCorrect != 0 || m.PolicyCorrect != 1 || m.ProductionCorrect != 0 ||
		m.ProductionUnderTier != 1 || m.RawUnderTier != 1 || m.PolicyUnderTier != 0 ||
		m.FrontierLabels != 1 || m.RawFrontierToSmall != 1 || m.PolicyFrontierToSmall != 0 {
		t.Fatalf("binary success must not become a minimum-tier label; safety must not hide raw errors: %+v", m)
	}
}
