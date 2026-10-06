// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package training

import (
	"sort"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// ShadowReport evaluates observations made before feedback. It does not
// rerun inference on reviewed tasks or claim candidate execution outcomes.
type ShadowReport struct {
	LoadErrors int                `json:"load_errors"`
	Invalid    int                `json:"invalid_observations"`
	Models     []ShadowModelStats `json:"models"`
}

// ShadowModelStats reports counts so small samples cannot look like evidence
// of generalization. Correctness uses explicit engineer-reviewed tier labels.
type ShadowModelStats struct {
	ModelID               string `json:"model_id"`
	Observed              int    `json:"observed"`
	Errors                int    `json:"errors"`
	Disagreements         int    `json:"policy_disagreements"`
	Upshifts              int    `json:"policy_upshifts"`
	Downshifts            int    `json:"policy_downshifts"`
	ReviewedOutcomes      int    `json:"reviewed_outcomes"`
	ProductionFailures    int    `json:"production_failures"`
	Labeled               int    `json:"explicit_tier_labels"`
	ProductionCorrect     int    `json:"production_recommendation_correct"`
	RawCorrect            int    `json:"raw_correct"`
	PolicyCorrect         int    `json:"policy_correct"`
	ProductionUnderTier   int    `json:"production_recommendation_under_tier"`
	RawUnderTier          int    `json:"raw_under_tier"`
	PolicyUnderTier       int    `json:"policy_under_tier"`
	FrontierLabels        int    `json:"frontier_labels"`
	RawFrontierToSmall    int    `json:"raw_frontier_to_small"`
	PolicyFrontierToSmall int    `json:"policy_frontier_to_small"`
}

// SummarizeShadow groups by the immutable artifact digest. selected_tier is
// the production recommendation, not proof the executor honored a rewrite.
func SummarizeShadow(events []Event) ShadowReport {
	report := ShadowReport{Models: []ShadowModelStats{}}
	models := map[string]*ShadowModelStats{}
	for _, e := range events {
		o := e.Shadow
		if o == nil {
			continue
		}
		if o.ErrorCode == "load_error" && o.ModelID == "" && o.Probabilities == nil {
			report.LoadErrors++
			continue
		}
		if !o.Valid() {
			report.Invalid++
			continue
		}
		m := models[o.ModelID]
		if m == nil {
			m = &ShadowModelStats{ModelID: o.ModelID}
			models[o.ModelID] = m
		}
		if o.ErrorCode != "" {
			m.Errors++
			continue
		}
		m.Observed++
		if o.PolicyTier != e.SelectedTier {
			m.Disagreements++
			if o.PolicyTier > e.SelectedTier {
				m.Upshifts++
			} else {
				m.Downshifts++
			}
		}
		if e.Outcome == nil || validateOutcome(*e.Outcome) != nil ||
			(e.Outcome.Retry && e.Outcome.RetryTier <= e.SelectedTier) {
			continue
		}
		m.ReviewedOutcomes++
		if e.Outcome.Failed {
			m.ProductionFailures++
		}
		if e.Outcome.RequiredTier == nil {
			continue
		}
		label := *e.Outcome.RequiredTier
		m.Labeled++
		if e.SelectedTier == label {
			m.ProductionCorrect++
		}
		if o.RawTier == label {
			m.RawCorrect++
		}
		if o.PolicyTier == label {
			m.PolicyCorrect++
		}
		if e.SelectedTier < label {
			m.ProductionUnderTier++
		}
		if o.RawTier < label {
			m.RawUnderTier++
		}
		if o.PolicyTier < label {
			m.PolicyUnderTier++
		}
		if label == core.TierFrontier {
			m.FrontierLabels++
			if o.RawTier == core.TierSmall {
				m.RawFrontierToSmall++
			}
			if o.PolicyTier == core.TierSmall {
				m.PolicyFrontierToSmall++
			}
		}
	}
	for _, m := range models {
		report.Models = append(report.Models, *m)
	}
	sort.Slice(report.Models, func(i, j int) bool { return report.Models[i].ModelID < report.Models[j].ModelID })
	return report
}
