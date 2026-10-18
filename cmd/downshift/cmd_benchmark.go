// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/benchmark"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routingv2/classifier"
	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
)

// runBenchmark runs the classifier against a labelled task dataset and prints
// a confusion matrix + false-downshift rate.
// Usage: downshift benchmark <dataset.json> [--compare] [--report] [--min-tier-accuracy=0.60]
func runBenchmark(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift benchmark <dataset.json> [--compare] [--report] [--min-tier-accuracy=0.60]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "The dataset is a JSON array of {\"prompt\":\"...\",\"label\":\"TRIVIAL|SIMPLE|MEDIUM|COMPLEX\"} objects")
		fmt.Fprintln(os.Stderr, "(tier labels SMALL|MID|FRONTIER are accepted too).")
		fmt.Fprintln(os.Stderr, "Maintainer benchmark files live in downshift-labs; public summary: benchmark/report.md.")
		fmt.Fprintln(os.Stderr, "Use --compare --candidate-weights=<file> to evaluate a candidate without activating it.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  --compare                Run both Legacy and CapabilityRouter v2 side by side")
		fmt.Fprintln(os.Stderr, "  --report                 Emit machine-readable JSON metrics summary to stdout")
		fmt.Fprintln(os.Stderr, "  --min-tier-accuracy=0.60 Exit with error (1) if tier routing accuracy is below threshold")
		fmt.Fprintln(os.Stderr, "  --gate                   Apply all gates (tier accuracy, FRONTIER→SMALL = 0, FRONTIER→MID max)")
		fmt.Fprintln(os.Stderr, "  --max-frontier-to-mid=0  With --gate: maximum FRONTIER→MID rate (default 0.70)")
		return 2
	}

	compare := false
	reportJSON := false
	gate := false
	minTierAccuracy := 0.0
	maxFrontierToMid := -1.0
	path := ""
	candidateWeights := ""
	for _, arg := range args {
		switch {
		case arg == "--compare":
			compare = true
		case arg == "--report":
			reportJSON = true
		case arg == "--gate":
			gate = true
		case strings.HasPrefix(arg, "--min-tier-accuracy="):
			valStr := strings.TrimPrefix(arg, "--min-tier-accuracy=")
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil || val < 0 || val > 1 {
				// A typo must not silently disable the gate.
				fmt.Fprintf(os.Stderr, "error: --min-tier-accuracy=%q must be a number between 0 and 1\n", valStr)
				return 2
			}
			minTierAccuracy = val
		case strings.HasPrefix(arg, "--max-frontier-to-mid="):
			valStr := strings.TrimPrefix(arg, "--max-frontier-to-mid=")
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil || val < 0 || val > 1 {
				fmt.Fprintf(os.Stderr, "error: --max-frontier-to-mid=%q must be a number between 0 and 1\n", valStr)
				return 2
			}
			maxFrontierToMid = val
		case strings.HasPrefix(arg, "--candidate-weights="):
			candidateWeights = strings.TrimPrefix(arg, "--candidate-weights=")
		default:
			path = arg
		}
	}

	if path == "" {
		fmt.Fprintln(os.Stderr, "error: dataset path required")
		return 2
	}

	if compare {
		return runBenchmarkCompare(path, candidateWeights)
	}

	tasks, err := benchmark.LoadDataset(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading dataset: %v\n", err)
		return 1
	}

	results := benchmark.Run(tasks, os.Stderr)
	if len(results) == 0 {
		// Every row was skipped (unknown labels) or the file is empty: a
		// report of zeros must not pass as a successful run.
		fmt.Fprintf(os.Stderr, "error: %s has no tasks with a recognised label (TRIVIAL|SIMPLE|MEDIUM|COMPLEX|SMALL|MID|FRONTIER)\n", path)
		return 1
	}
	report := benchmark.GenerateReport(results)

	if reportJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding report: %v\n", err)
			return 1
		}
	} else if !gate {
		matrix := benchmark.Build(results)
		benchmark.Print(results, matrix, os.Stdout)
	}

	gates := benchmark.DefaultGateThresholds()
	if minTierAccuracy > 0 {
		gates.MinTierAccuracy = minTierAccuracy
	}
	if maxFrontierToMid >= 0 {
		gates.MaxFrontierToMIDRate = maxFrontierToMid
	}
	if gate {
		ok, reasons := benchmark.EvaluateGates(report, gates)
		if !ok {
			for _, r := range reasons {
				fmt.Fprintf(os.Stderr, "benchmark gate failed: %s\n", r)
			}
			return 1
		}
	} else if minTierAccuracy > 0 && report.TierAccuracy < minTierAccuracy {
		fmt.Fprintf(os.Stderr, "benchmark gate failed: tier accuracy %.1f%% below minimum required %.1f%%\n",
			report.TierAccuracy*100, minTierAccuracy*100)
		return 1
	}

	return 0
}

// runBenchmarkCompare runs Legacy vs CapabilityRouter v2 side by side.
// Uses the v2 training dataset format (SMALL/MID/FRONTIER labels).
func runBenchmarkCompare(path, candidateWeights string) int {
	// Load dataset in v2 format (SMALL/MID/FRONTIER)
	v2ds, err := training.LoadDataset(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading v2 dataset: %v\n", err)
		return 1
	}
	if v2ds.Size() == 0 {
		fmt.Fprintln(os.Stderr, "error: comparison dataset must contain at least one task")
		return 2
	}

	legacyTasks, err := legacyTasksForComparison(v2ds.Examples)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	// Legacy: map equivalent tiers to the old classifier's label vocabulary.
	legacyResults := benchmark.Run(legacyTasks, os.Stderr)

	// v2: run the capability router classifier
	clf := classifier.NewSoftmaxClassifierDefault()
	if candidateWeights != "" {
		weights, err := classifier.LoadWeightsFromFile(candidateWeights)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading candidate weights: %v\n", err)
			return 1
		}
		clf = classifier.NewSoftmaxClassifier(weights)
	}
	v2Actual := make([]core.Tier, v2ds.Size())
	v2Predicted := make([]core.Tier, v2ds.Size())

	for i, ex := range v2ds.Examples {
		v2Actual[i] = ex.Tier()
		probs, _ := clf.Classify(ex.Features)
		v2Predicted[i] = probs.MaxTier()
	}

	v2Metrics := training.Compute(v2Actual, v2Predicted)
	classCounts := map[core.Tier]int{}
	for _, tier := range v2Actual {
		classCounts[tier]++
	}

	// Compute legacy metrics in v2 terms (map COMPLEX→FRONTIER, etc.)
	legacyActual := make([]core.Tier, len(legacyResults))
	legacyPredicted := make([]core.Tier, len(legacyResults))
	for i, r := range legacyResults {
		// Convert legacy complexity to tier
		switch r.Task.Label {
		case "TRIVIAL":
			legacyActual[i] = core.TierSmall
		case "SIMPLE", "MEDIUM":
			legacyActual[i] = core.TierMid
		case "COMPLEX":
			legacyActual[i] = core.TierFrontier
		default:
			legacyActual[i] = core.TierMid
		}
		legacyPredicted[i] = core.TierOf(r.Predicted)
	}
	legacyMetrics := training.Compute(legacyActual, legacyPredicted)

	// Print comparison
	fmt.Fprintln(os.Stdout, "Benchmark: Legacy vs Capability Router v2")
	if candidateWeights != "" {
		fmt.Fprintf(os.Stdout, "Candidate weights: %s\n", candidateWeights)
	}
	fmt.Fprintf(os.Stdout, "Dataset: %d tasks\n\n", v2ds.Size())
	fmt.Fprintf(os.Stdout, "Label support: SMALL=%d MID=%d FRONTIER=%d\n", classCounts[core.TierSmall], classCounts[core.TierMid], classCounts[core.TierFrontier])
	if classCounts[core.TierSmall] < 5 || classCounts[core.TierMid] < 5 || classCounts[core.TierFrontier] < 5 {
		fmt.Fprintln(os.Stdout, "Warning: fewer than 5 examples in at least one tier; treat this comparison as exploratory.")
	}

	modelLabel := "v2"
	if candidateWeights != "" {
		modelLabel = "Candidate"
	}
	fmt.Fprintf(os.Stdout, "%-35s %10s %10s %10s\n", "", "Legacy", modelLabel, "Delta")
	fmt.Fprintf(os.Stdout, "%-35s %10.1f%% %10.1f%% %+9.1f%%\n",
		"Tier accuracy",
		legacyMetrics.Accuracy*100, v2Metrics.Accuracy*100,
		(v2Metrics.Accuracy-legacyMetrics.Accuracy)*100)
	fmt.Fprintf(os.Stdout, "%-35s %10.1f%% %10.1f%% %+9.1f%%\n",
		"Unsafe downgrade",
		legacyMetrics.UnsafeDowngradeRate*100, v2Metrics.UnsafeDowngradeRate*100,
		(v2Metrics.UnsafeDowngradeRate-legacyMetrics.UnsafeDowngradeRate)*100)
	fmt.Fprintf(os.Stdout, "  %-33s %10d  %10d\n",
		"FRONTIER → SMALL",
		legacyMetrics.Unsafe.FrontierToSmall, v2Metrics.Unsafe.FrontierToSmall)
	fmt.Fprintf(os.Stdout, "  %-33s %10d  %10d\n",
		"FRONTIER → MID",
		legacyMetrics.Unsafe.FrontierToMid, v2Metrics.Unsafe.FrontierToMid)
	fmt.Fprintf(os.Stdout, "%-35s %10.1f%% %10.1f%% %+9.1f%%\n",
		"Over-routing",
		legacyMetrics.OverRoutingRate*100, v2Metrics.OverRoutingRate*100,
		(v2Metrics.OverRoutingRate-legacyMetrics.OverRoutingRate)*100)
	fmt.Fprintf(os.Stdout, "%-35s %10.3f  %10.3f  %+9.3f\n",
		"Risk-weighted loss",
		legacyMetrics.RiskWeightedLoss, v2Metrics.RiskWeightedLoss,
		v2Metrics.RiskWeightedLoss-legacyMetrics.RiskWeightedLoss)
	fmt.Fprintln(os.Stdout, "")

	// Verdict
	if v2Metrics.Unsafe.FrontierToSmall < legacyMetrics.Unsafe.FrontierToSmall ||
		v2Metrics.UnsafeDowngradeRate < legacyMetrics.UnsafeDowngradeRate {
		fmt.Fprintln(os.Stdout, "Recommendation: Capability v2 reduces unsafe downgrades.")
		fmt.Fprintln(os.Stdout, "                Accept any over-routing increase for safety gain.")
	} else if v2Metrics.Accuracy > legacyMetrics.Accuracy {
		fmt.Fprintln(os.Stdout, "Recommendation: Capability v2 improves overall accuracy.")
	} else {
		fmt.Fprintln(os.Stdout, "Recommendation: No clear improvement — keep legacy as default.")
		fmt.Fprintln(os.Stdout, "                Collect more training data and re-run train + compare.")
	}

	return 0
}

func legacyTasksForComparison(examples []training.Example) ([]benchmark.Task, error) {
	labels := map[string]string{
		"SMALL":    "TRIVIAL",
		"MID":      "MEDIUM",
		"FRONTIER": "COMPLEX",
	}
	tasks := make([]benchmark.Task, 0, len(examples))
	for _, example := range examples {
		label, ok := labels[strings.ToUpper(strings.TrimSpace(example.Label))]
		if !ok {
			return nil, fmt.Errorf("unsupported tier %q; use SMALL, MID, or FRONTIER", example.Label)
		}
		tasks = append(tasks, benchmark.Task{Prompt: example.Prompt, Label: label})
	}
	return tasks, nil
}

// runTrain trains the capability router v2 classifier on a labelled dataset.
// Usage: downshift train <dataset.json> [--output=weights.json] [--validation-split=0.2] [--from-events]
func runTrain(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift train <dataset.json> [--output=<path>] [--validation-split=0.2]")
		fmt.Fprintln(os.Stderr, "       downshift train --from-events [--output=<path>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Dataset format: [{\"prompt\":\"...\",\"label\":\"SMALL|MID|FRONTIER\"}, ...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  --output=<path>          Where to write weights.json (default: ./weights.json)")
		fmt.Fprintln(os.Stderr, "  --validation-split=0.2   Fraction to use for validation (default: 0.2)")
		fmt.Fprintln(os.Stderr, "  --from-events            Train from collected routing events instead of a dataset")
		fmt.Fprintln(os.Stderr, "  --epochs=100             Number of training epochs (default: 100)")
		fmt.Fprintln(os.Stderr, "  --learning-rate=0.1      Learning rate for gradient descent (default: 0.1)")
		return 2
	}

	// Parse flags
	var datasetPath string
	outputPath := "weights.json"
	validationSplit := 0.2
	fromEvents := false
	config := training.DefaultTrainConfig()

	for _, arg := range args {
		switch {
		case arg == "--from-events":
			fromEvents = true
		case strings.HasPrefix(arg, "--output=") && len(arg) > len("--output="):
			outputPath = strings.TrimPrefix(arg, "--output=")
		case strings.HasPrefix(arg, "--validation-split=") && len(arg) > len("--validation-split="):
			v, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--validation-split="), 64)
			if err == nil && v > 0 && v < 1 {
				validationSplit = v
			}
		case strings.HasPrefix(arg, "--epochs=") && len(arg) > len("--epochs="):
			v, err := strconv.Atoi(strings.TrimPrefix(arg, "--epochs="))
			if err == nil && v > 0 {
				config.Epochs = v
			}
		case strings.HasPrefix(arg, "--learning-rate=") && len(arg) > len("--learning-rate="):
			v, err := strconv.ParseFloat(strings.TrimPrefix(arg, "--learning-rate="), 64)
			if err == nil && v > 0 {
				config.LearningRate = v
			}
		default:
			if !fromEvents {
				datasetPath = arg
			}
		}
	}

	fmt.Fprintln(os.Stdout, "Training capability-router v2")
	fmt.Fprintln(os.Stdout, "")

	var result training.TrainResult
	var err error

	if fromEvents {
		eventsPath := training.DefaultEventsPath()
		fmt.Fprintf(os.Stdout, "Source:  events file (%s)\n", eventsPath)
		store := training.NewEventStore(eventsPath)
		if _, err = store.Load(); err == nil {
			labeled := store.ToDataset()
			fmt.Fprintf(os.Stdout, "Reviewed minimum-tier labels: %d\n", labeled.Size())
			trainDS, valDS := labeled.Split(validationSplit)
			fmt.Fprintf(os.Stdout, "Train/Val: %d / %d\n\n", trainDS.Size(), valDS.Size())
			result = training.Train(trainDS, valDS, config)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading events: %v\n", err)
			return 1
		}
		if result.Weights == nil {
			fmt.Fprintln(os.Stderr, "no reviewed minimum-tier labels yet — add --required-tier=SMALL|MID|FRONTIER when recording feedback")
			return 1
		}
	} else {
		if datasetPath == "" {
			fmt.Fprintln(os.Stderr, "error: dataset path required (or use --from-events)")
			return 2
		}

		ds, loadErr := training.LoadDataset(datasetPath)
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "error loading dataset: %v\n", loadErr)
			return 1
		}

		fmt.Fprintf(os.Stdout, "Dataset: %d tasks\n", ds.Size())
		trainDS, valDS := ds.Split(validationSplit)
		fmt.Fprintf(os.Stdout, "Train/Val: %d / %d\n\n", trainDS.Size(), valDS.Size())

		result = training.Train(trainDS, valDS, config)
	}

	// Print training results
	fmt.Fprintf(os.Stdout, "Epochs: %d  LR: %.3f  Risk-weighted: %v\n\n",
		config.Epochs, config.LearningRate, config.RiskWeighted)
	fmt.Fprintf(os.Stdout, "Training metrics:\n%s\n", result.TrainMetrics.Summary())
	if result.ValMetrics.ConfusionMatrix.Total() > 0 {
		fmt.Fprintf(os.Stdout, "Validation metrics:\n%s\n", result.ValMetrics.Summary())
	}

	// Save weights
	if err := classifier.SaveWeights(result.Weights, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "error saving weights: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "Weights saved to: %s\n", outputPath)
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintf(os.Stdout, "To evaluate: downshift benchmark <holdout.json> --compare --candidate-weights=%s\n", outputPath)
	fmt.Fprintln(os.Stdout, "Candidate weights are not activated automatically; review safety and quality metrics before promotion.")
	return 0
}
