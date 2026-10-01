// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

// Package benchmark evaluates the classifier against a hand-labelled task
// dataset and reports four KPIs aligned to the routing problem:
//
//   - Complexity accuracy — exact label match (TRIVIAL/SIMPLE/MEDIUM/COMPLEX)
//   - Tier routing accuracy — what matters economically: did the task get the
//     right model tier? SIMPLE→MID and MEDIUM→MID are both correct routing.
//   - Unsafe downgrade rate — FRONTIER tasks sent to a cheaper tier; the
//     safety-critical metric. A COMPLEX task on a weak model can produce wrong
//     output worth far more in rework than the cost saved.
//   - Wasteful over-routing rate — SMALL tasks sent to a dearer tier; wastes
//     money but is safe.
//
// Dataset format (JSON):
//
//	[
//	  { "prompt": "rename the userId variable", "label": "TRIVIAL" },
//	  { "prompt": "rearchitect the auth module", "label": "COMPLEX" }
//	]
//
// Labels are case-insensitive: TRIVIAL, SIMPLE, MEDIUM, COMPLEX.
package benchmark

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

// Task is one entry in the benchmark dataset.
type Task struct {
	Prompt string `json:"prompt"`
	Label  string `json:"label"` // expected complexity: TRIVIAL|SIMPLE|MEDIUM|COMPLEX
}

// Result holds the outcome for one task.
type Result struct {
	Task      Task
	Predicted core.Complexity
	Correct   bool
}

// complexityFromString parses a label string into a Complexity.
// Returns Medium and false when the label is unknown.
func complexityFromString(s string) (core.Complexity, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRIVIAL":
		return core.Trivial, true
	case "SIMPLE":
		return core.Simple, true
	case "MEDIUM":
		return core.Medium, true
	case "COMPLEX":
		return core.Complex, true
	default:
		return core.Medium, false
	}
}

// normalizePrompt returns the canonical form of a prompt used for
// duplicate detection: trimmed of surrounding whitespace and lowercased.
// Two prompts that differ only in case or surrounding whitespace are
// considered the same task.
func normalizePrompt(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// LoadDataset reads a JSON task dataset from path.
//
// Validation: LoadDataset rejects files that cannot be read, files that
// are not valid JSON, and datasets containing duplicate prompts
// (compared case-insensitively after trimming whitespace). Unknown labels
// and empty prompts are intentionally NOT rejected here: unknown labels
// are skipped later by Run (with a log line) so a single bad row never
// fails the whole run, and empty prompts are surfaced via DatasetHealth
// instead of hard-failing. This keeps LoadDataset backward compatible
// with the current 30-task seed file.
func LoadDataset(path string) ([]Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading dataset: %w", err)
	}
	var tasks []Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parsing dataset: %w", err)
	}
	// Reject duplicate prompts: the same prompt twice would double-count
	// one task and silently skew accuracy, so fail fast with the index.
	seen := make(map[string]int, len(tasks))
	for i, t := range tasks {
		norm := normalizePrompt(t.Prompt)
		if norm == "" {
			continue
		}
		if first, ok := seen[norm]; ok {
			return nil, fmt.Errorf("duplicate prompt at index %d: %q (first seen at index %d)", i, t.Prompt, first)
		}
		seen[norm] = i
	}
	return tasks, nil
}

// HealthReport summarises the structural health of a task dataset without
// running the classifier. Use it to audit a dataset before trusting its
// accuracy numbers: an imbalanced label mix or duplicated rows can make
// the KPIs misleading.
type HealthReport struct {
	Total int
	// CountsByLabel holds per-label counts keyed by normalised label
	// (TRIVIAL, SIMPLE, MEDIUM, COMPLEX). Rows with any other label
	// (including empty) are counted in Unknown instead.
	CountsByLabel map[string]int
	// Unknown counts rows whose label is not one of the four known labels.
	Unknown int
	// EmptyCount counts rows whose prompt is empty or whitespace-only.
	EmptyCount int
	// DuplicateCount counts rows whose normalised prompt (case-insensitive
	// trim) already appeared at an earlier index. Only the repeats are
	// counted, not the first occurrence.
	DuplicateCount int
}

// DatasetHealth reports label distribution, empty prompts, and duplicate
// prompts for tasks. It never returns an error and never skips rows: every
// input task is counted exactly once, so callers can audit raw datasets
// (including ones LoadDataset would reject for duplicates).
func DatasetHealth(tasks []Task) HealthReport {
	report := HealthReport{
		Total:         len(tasks),
		CountsByLabel: map[string]int{"TRIVIAL": 0, "SIMPLE": 0, "MEDIUM": 0, "COMPLEX": 0},
	}
	seen := make(map[string]struct{}, len(tasks))
	for _, t := range tasks {
		if strings.TrimSpace(t.Prompt) == "" {
			report.EmptyCount++
		} else {
			norm := normalizePrompt(t.Prompt)
			if _, ok := seen[norm]; ok {
				report.DuplicateCount++
			} else {
				seen[norm] = struct{}{}
			}
		}
		label := strings.ToUpper(strings.TrimSpace(t.Label))
		switch label {
		case "TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX":
			report.CountsByLabel[label]++
		default:
			report.Unknown++
		}
	}
	return report
}

// Run classifies every task with ClassifyWithSemantic (regex + optional MiniLM).
// Tasks with unrecognised labels are skipped (logged to w).
func Run(tasks []Task, w io.Writer) []Result {
	return RunWithClassifier(tasks, core.ClassifyWithSemantic, w)
}

// RunWithClassifier evaluates tasks using a custom classify function.
func RunWithClassifier(tasks []Task, classify func(string) core.Classification, w io.Writer) []Result {
	var results []Result
	skipped := 0
	for _, t := range tasks {
		expected, ok := complexityFromString(t.Label)
		if !ok {
			fmt.Fprintf(w, "skip: unknown label %q for prompt %q\n", t.Label, t.Prompt)
			skipped++
			continue
		}
		cls := classify(t.Prompt)
		results = append(results, Result{
			Task:      t,
			Predicted: cls.Complexity,
			Correct:   cls.Complexity == expected,
		})
	}
	if skipped > 0 {
		fmt.Fprintf(w, "skipped %d tasks with unrecognised labels\n\n", skipped)
	}
	return results
}

// Matrix is a 4×4 confusion matrix indexed by [actual][predicted].
// Row = actual label, Column = predicted label.
// Order: Trivial=0, Simple=1, Medium=2, Complex=3.
type Matrix [4][4]int

// Build computes the confusion matrix from results.
func Build(results []Result) Matrix {
	var m Matrix
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if !ok {
			continue
		}
		m[expected][r.Predicted]++
	}
	return m
}

// Accuracy returns the fraction of exact complexity-label matches.
func Accuracy(results []Result) float64 {
	if len(results) == 0 {
		return 0
	}
	correct := 0
	for _, r := range results {
		if r.Correct {
			correct++
		}
	}
	return float64(correct) / float64(len(results))
}

// TierAccuracy returns the fraction of tasks where the predicted complexity
// maps to the correct model tier. This is the economically meaningful metric:
// SIMPLE→MID and MEDIUM→MID are different complexity labels but identical
// routing decisions, so both count as correct tier routing.
func TierAccuracy(results []Result) float64 {
	if len(results) == 0 {
		return 0
	}
	correct := 0
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if !ok {
			continue
		}
		if expected.Tier() == r.Predicted.Tier() {
			correct++
		}
	}
	return float64(correct) / float64(len(results))
}

// UnsafeDowngradeRates returns the fraction of FRONTIER-tier tasks (COMPLEX)
// mis-routed to MID or SMALL. These are the safety-critical failures.
// Returns (toMID rate, toSMALL rate, count-to-mid, count-to-small, total).
func UnsafeDowngradeRates(results []Result) (toMID, toSMALL float64, cntMID, cntSMALL, total int) {
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if !ok || expected.Tier() != core.TierFrontier {
			continue
		}
		total++
		switch r.Predicted.Tier() {
		case core.TierMid:
			cntMID++
		case core.TierSmall:
			cntSMALL++
		}
	}
	if total == 0 {
		return 0, 0, 0, 0, 0
	}
	return float64(cntMID) / float64(total),
		float64(cntSMALL) / float64(total),
		cntMID, cntSMALL, total
}

// WastefulOverRoutingRates returns the fraction of SMALL-tier tasks (TRIVIAL)
// mis-routed to MID or FRONTIER. Wasteful but safe.
// Returns (toMID rate, toFRONTIER rate, count-to-mid, count-to-frontier, total).
func WastefulOverRoutingRates(results []Result) (toMID, toFrontier float64, cntMID, cntFrontier, total int) {
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if !ok || expected.Tier() != core.TierSmall {
			continue
		}
		total++
		switch r.Predicted.Tier() {
		case core.TierMid:
			cntMID++
		case core.TierFrontier:
			cntFrontier++
		}
	}
	if total == 0 {
		return 0, 0, 0, 0, 0
	}
	return float64(cntMID) / float64(total),
		float64(cntFrontier) / float64(total),
		cntMID, cntFrontier, total
}

// FalseDownshiftRate returns the fraction of COMPLEX tasks routed to any
// cheaper tier. Kept for backward compatibility; prefer UnsafeDowngradeRates
// for the split MID/SMALL breakdown.
func FalseDownshiftRate(results []Result) (rate float64, count int, total int) {
	_, _, cntMID, cntSMALL, tot := UnsafeDowngradeRates(results)
	count = cntMID + cntSMALL
	total = tot
	if total == 0 {
		return 0, 0, 0
	}
	return float64(count) / float64(total), count, total
}

// OverRoutingRate returns the fraction of TRIVIAL tasks routed to a more
// expensive tier. Kept for backward compatibility.
func OverRoutingRate(results []Result) (rate float64, count int, total int) {
	_, _, cntMID, cntFrontier, tot := WastefulOverRoutingRates(results)
	count = cntMID + cntFrontier
	total = tot
	if total == 0 {
		return 0, 0, 0
	}
	return float64(count) / float64(total), count, total
}

var labels = []string{"T", "S", "M", "C"}
var fullLabels = []string{"TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"}

// Print writes the full benchmark report to w.
func Print(results []Result, m Matrix, w io.Writer) {
	n := len(results)
	complexAcc := Accuracy(results)
	tierAcc := TierAccuracy(results)

	udMID, udSMALL, udCntMID, udCntSMALL, udTotal := UnsafeDowngradeRates(results)
	worMID, worFrontier, worCntMID, worCntFrontier, worTotal := WastefulOverRoutingRates(results)

	fmt.Fprintf(w, "Dataset: %d tasks\n\n", n)

	// Four KPIs aligned to the routing problem.
	fmt.Fprintf(w, "Complexity accuracy   %6.1f%%  (exact label match)\n", complexAcc*100)
	fmt.Fprintf(w, "Tier routing accuracy %6.1f%%  (correct model tier — what matters economically)\n", tierAcc*100)
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "Unsafe downgrade (FRONTIER → cheaper tier):\n")
	if udTotal == 0 {
		fmt.Fprintf(w, "  No FRONTIER-tier tasks in dataset\n")
	} else {
		fmt.Fprintf(w, "  FRONTIER → MID      %6.1f%%  (%d / %d)\n", udMID*100, udCntMID, udTotal)
		fmt.Fprintf(w, "  FRONTIER → SMALL    %6.1f%%  (%d / %d)\n", udSMALL*100, udCntSMALL, udTotal)
	}
	fmt.Fprintf(w, "\n")

	fmt.Fprintf(w, "Wasteful over-routing (SMALL → dearer tier):\n")
	if worTotal == 0 {
		fmt.Fprintf(w, "  No SMALL-tier tasks in dataset\n")
	} else {
		fmt.Fprintf(w, "  SMALL → MID         %6.1f%%  (%d / %d)\n", worMID*100, worCntMID, worTotal)
		fmt.Fprintf(w, "  SMALL → FRONTIER    %6.1f%%  (%d / %d)\n", worFrontier*100, worCntFrontier, worTotal)
	}
	fmt.Fprintf(w, "\n")

	// Detail: which FRONTIER tasks were mis-routed (the safety failures).
	hasMisrouted := false
	for _, r := range results {
		expected, ok := complexityFromString(r.Task.Label)
		if ok && expected.Tier() == core.TierFrontier && r.Predicted.Tier() != core.TierFrontier {
			hasMisrouted = true
			break
		}
	}
	if hasMisrouted {
		fmt.Fprintf(w, "Unsafe downgrade detail (FRONTIER → cheaper tier)\n")
		for _, r := range results {
			expected, ok := complexityFromString(r.Task.Label)
			if !ok || expected.Tier() != core.TierFrontier || r.Predicted.Tier() == core.TierFrontier {
				continue
			}
			fmt.Fprintf(w, "  COMPLEX → %-7s  %q\n",
				r.Predicted.String(), truncate(r.Task.Prompt, 70))
		}
		fmt.Fprintf(w, "\n")
	}

	// Confusion matrix.
	fmt.Fprintf(w, "Confusion matrix\n\n")
	fmt.Fprintf(w, "             ")
	for _, l := range labels {
		fmt.Fprintf(w, "  %4s", l)
	}
	fmt.Fprintf(w, "   (predicted)\n")
	for row, rowLabel := range fullLabels {
		fmt.Fprintf(w, "Actual %-7s", rowLabel)
		for col := range labels {
			fmt.Fprintf(w, "  %4d", m[row][col])
		}
		fmt.Fprintf(w, "\n")
	}
	fmt.Fprintf(w, "\nT=TRIVIAL  S=SIMPLE  M=MEDIUM  C=COMPLEX\n")
	fmt.Fprintf(w, "Tiers: TRIVIAL→SMALL  SIMPLE→MID  MEDIUM→MID  COMPLEX→FRONTIER\n")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// ConfidenceInterval holds the 95% bootstrap confidence bounds [Low, High].
type ConfidenceInterval struct {
	Low  float64 `json:"low"`
	High float64 `json:"high"`
}

// BootstrapConfidenceInterval computes a 95% bootstrap confidence interval
// for TierAccuracy using B resamples.
func BootstrapConfidenceInterval(results []Result, resamples int) ConfidenceInterval {
	n := len(results)
	if n == 0 {
		return ConfidenceInterval{}
	}
	if resamples <= 0 {
		resamples = 1000
	}

	accuracies := make([]float64, resamples)
	// Deterministic LCG pseudo-random generator for reproducibility without external deps
	seed := uint64(133742)
	lcgNext := func() int {
		seed = seed*6364136223846793005 + 1
		return int((seed >> 32) % uint64(n))
	}

	for b := 0; b < resamples; b++ {
		correct := 0
		for i := 0; i < n; i++ {
			idx := lcgNext()
			r := results[idx]
			expected, ok := complexityFromString(r.Task.Label)
			if ok && expected.Tier() == r.Predicted.Tier() {
				correct++
			}
		}
		accuracies[b] = float64(correct) / float64(n)
	}

	// Simple sort for percentiles
	for i := 0; i < len(accuracies)-1; i++ {
		for j := i + 1; j < len(accuracies); j++ {
			if accuracies[j] < accuracies[i] {
				accuracies[i], accuracies[j] = accuracies[j], accuracies[i]
			}
		}
	}

	lowIdx := int(float64(resamples) * 0.025)
	highIdx := int(float64(resamples) * 0.975)
	if highIdx >= resamples {
		highIdx = resamples - 1
	}

	return ConfidenceInterval{
		Low:  accuracies[lowIdx],
		High: accuracies[highIdx],
	}
}

// Report holds computed benchmark metrics for JSON export or programmatic gating.
type Report struct {
	TotalTasks               int                 `json:"total_tasks"`
	ComplexityAccuracy       float64             `json:"complexity_accuracy"`
	TierAccuracy             float64             `json:"tier_accuracy"`
	TierAccuracyCI95         ConfidenceInterval  `json:"tier_accuracy_ci_95"`
	FrontierToMidRate        float64             `json:"frontier_to_mid_rate"`
	FrontierToSmallRate      float64             `json:"frontier_to_small_rate"`
	FrontierTotal            int                 `json:"frontier_total"`
	SmallToMidRate           float64             `json:"small_to_mid_rate"`
	SmallToFrontierRate      float64             `json:"small_to_frontier_rate"`
	SmallTotal               int                 `json:"small_total"`
}

// GateThresholds defines CI regression limits.
type GateThresholds struct {
	MinTierAccuracy          float64
	MaxFrontierToMIDRate     float64
	MaxFrontierToSmallRate   float64
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
