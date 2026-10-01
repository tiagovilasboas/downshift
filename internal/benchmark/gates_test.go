// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1

package benchmark_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/benchmark"
	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

func TestEvaluateGates_FailsOnFrontierToSmall(t *testing.T) {
	results := []benchmark.Result{{
		Task:      benchmark.Task{Label: "COMPLEX"},
		Predicted: core.Trivial,
	}}
	rep := benchmark.GenerateReport(results)
	ok, reasons := benchmark.EvaluateGates(rep, benchmark.DefaultGateThresholds())
	if ok || len(reasons) == 0 {
		t.Fatal("expected gate failure for COMPLEX→SMALL")
	}
}

func TestEvaluateGates_PassesHealthyReport(t *testing.T) {
	results := []benchmark.Result{{
		Task:      benchmark.Task{Label: "TRIVIAL"},
		Predicted: core.Trivial,
	}}
	rep := benchmark.GenerateReport(results)
	gates := benchmark.GateThresholds{MinTierAccuracy: 0.5, MaxFrontierToMIDRate: 1, MaxFrontierToSmallRate: 0}
	ok, _ := benchmark.EvaluateGates(rep, gates)
	if !ok {
		t.Fatal("expected pass for trivial correct routing")
	}
}

func TestGenerateReport_HasBootstrapCI(t *testing.T) {
	results := []benchmark.Result{
		{Task: benchmark.Task{Label: "SIMPLE"}, Predicted: core.Simple},
		{Task: benchmark.Task{Label: "COMPLEX"}, Predicted: core.Complex},
	}
	rep := benchmark.GenerateReport(results)
	if rep.TierAccuracyCI95.Low < 0 || rep.TierAccuracyCI95.High > 1.01 {
		t.Fatalf("unexpected CI: %+v", rep.TierAccuracyCI95)
	}
}
