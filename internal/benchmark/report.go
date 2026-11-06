// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark

import "fmt"

// Report holds computed benchmark metrics for JSON export or programmatic gating.
type Report struct {
	TotalTasks          int                `json:"total_tasks"`
	ComplexityAccuracy  float64            `json:"complexity_accuracy"`
	TierAccuracy        float64            `json:"tier_accuracy"`
	TierAccuracyCI95    ConfidenceInterval `json:"tier_accuracy_ci_95"`
	FrontierToMidRate   float64            `json:"frontier_to_mid_rate"`
	FrontierToSmallRate float64            `json:"frontier_to_small_rate"`
	FrontierTotal       int                `json:"frontier_total"`
	SmallToMidRate      float64            `json:"small_to_mid_rate"`
	SmallToFrontierRate float64            `json:"small_to_frontier_rate"`
	SmallTotal          int                `json:"small_total"`
}

// GateThresholds defines CI regression limits.
type GateThresholds struct {
	MinTierAccuracy        float64
	MaxFrontierToMIDRate   float64
	MaxFrontierToSmallRate float64
}

// DefaultGateThresholds is the seed-dataset regression bar (directional).
func DefaultGateThresholds() GateThresholds {
	return GateThresholds{
		MinTierAccuracy:        0.55,
		MaxFrontierToMIDRate:   0.70,
		MaxFrontierToSmallRate: 0.0,
	}
}

// EvaluateGates returns false and reasons when metrics violate thresholds.
func EvaluateGates(rep Report, gates GateThresholds) (bool, []string) {
	var reasons []string
	if rep.TierAccuracy < gates.MinTierAccuracy {
		reasons = append(reasons, fmt.Sprintf("tier_accuracy %.3f < min %.3f", rep.TierAccuracy, gates.MinTierAccuracy))
	}
	if rep.FrontierToSmallRate > gates.MaxFrontierToSmallRate {
		reasons = append(reasons, fmt.Sprintf("frontier_to_small_rate %.3f > max %.3f", rep.FrontierToSmallRate, gates.MaxFrontierToSmallRate))
	}
	if rep.FrontierToMidRate > gates.MaxFrontierToMIDRate {
		reasons = append(reasons, fmt.Sprintf("frontier_to_mid_rate %.3f > max %.3f", rep.FrontierToMidRate, gates.MaxFrontierToMIDRate))
	}
	return len(reasons) == 0, reasons
}

// GenerateReport computes summary metrics from results.
func GenerateReport(results []Result) Report {
	udMID, udSMALL, _, _, udTotal := UnsafeDowngradeRates(results)
	worMID, worFrontier, _, _, worTotal := WastefulOverRoutingRates(results)

	return Report{
		TotalTasks:          len(results),
		ComplexityAccuracy:  Accuracy(results),
		TierAccuracy:        TierAccuracy(results),
		TierAccuracyCI95:    BootstrapConfidenceInterval(results, 1000),
		FrontierToMidRate:   udMID,
		FrontierToSmallRate: udSMALL,
		FrontierTotal:       udTotal,
		SmallToMidRate:      worMID,
		SmallToFrontierRate: worFrontier,
		SmallTotal:          worTotal,
	}
}
