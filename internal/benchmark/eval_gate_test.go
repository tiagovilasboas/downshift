// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/benchmark"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// The classifier regression gate. It replaces the `downshift benchmark --gate`
// runs the labs CI made before the command was removed (#589): the same two
// labelled sets, copied from downshift-labs eval/benchmark (seed = tasks.json,
// holdout = holdout.json), with the same bars.
func TestClassifierEvalGate(t *testing.T) {
	cases := []struct {
		file    string
		minTier float64
	}{
		{"seed.json", 0.60},
		{"holdout.json", 0.80},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			tasks, err := benchmark.LoadDataset(filepath.Join("testdata", "eval", c.file))
			if err != nil {
				t.Fatal(err)
			}
			rep := benchmark.GenerateReport(benchmark.Run(tasks, io.Discard))
			gates := benchmark.GateThresholds{MinTierAccuracy: c.minTier, MaxFrontierToMIDRate: 0, MaxFrontierToSmallRate: 0}
			if ok, reasons := benchmark.EvaluateGates(rep, gates); !ok {
				t.Fatalf("%s fails the gate: %s", c.file, strings.Join(reasons, "; "))
			}
		})
	}
}

// The class-routing safety gate: a spawn the classifier routes to small
// (a decisive TRIVIAL or SIMPLE call with no risk category, decide precedence
// 5) must never be work whose labelled tier is above small. The two sets above
// always run; DOWNSHIFT_EVAL_DIR adds the eval-only sets of downshift-labs
// (fresh, heldout2, blind-vitrine), which never live in this repository.
func TestClassRoutingNeverRoutesHarderWorkToSmall(t *testing.T) {
	sets := []string{filepath.Join("testdata", "eval", "seed.json"), filepath.Join("testdata", "eval", "holdout.json")}
	if dir := os.Getenv("DOWNSHIFT_EVAL_DIR"); dir != "" {
		for _, f := range []string{"fresh.json", "heldout2.json", "blind-vitrine.json"} {
			sets = append(sets, filepath.Join(dir, f))
		}
	}
	for _, path := range sets {
		tasks, err := benchmark.LoadDataset(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		moved := 0
		for _, task := range tasks {
			truth, ok := tierOfLabel(task.Label)
			if !ok {
				continue
			}
			cls := core.ClassifyWithSemantic(task.Prompt)
			if !cls.Decisive() || len(core.RiskCategories(task.Prompt)) > 0 || core.TierOf(cls.Complexity) != core.TierSmall {
				continue
			}
			moved++
			if truth != core.TierSmall {
				t.Errorf("%s: routed to small but labelled %s: %q", filepath.Base(path), task.Label, task.Prompt)
			}
		}
		t.Logf("%s: %d of %d tasks would move to small", filepath.Base(path), moved, len(tasks))
	}
}

func tierOfLabel(label string) (core.Tier, bool) {
	switch strings.ToUpper(strings.TrimSpace(label)) {
	case "TRIVIAL", "SIMPLE", "SMALL":
		return core.TierSmall, true
	case "MEDIUM", "MID":
		return core.TierMid, true
	case "COMPLEX", "FRONTIER":
		return core.TierFrontier, true
	}
	return core.TierSmall, false
}
