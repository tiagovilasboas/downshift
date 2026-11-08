// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package benchmark evaluates the classifier against a hand-labelled task
// dataset and reports four KPIs aligned to the routing problem:
//
//   - Complexity accuracy — exact label match (TRIVIAL/SIMPLE/MEDIUM/COMPLEX)
//   - Tier routing accuracy — what matters economically: did the task get the
//     right model tier? TRIVIAL and SIMPLE are small. MEDIUM is mid. COMPLEX is frontier.
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
// Labels are case-insensitive: TRIVIAL, SIMPLE, MEDIUM, COMPLEX, or the tier
// labels SMALL, MID, FRONTIER.
package benchmark

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
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
	// Confident and NoSignal copy the classification. An unconfident result
	// is an abstention: the router keeps the requested model for it.
	Confident bool
	NoSignal  bool
}

// complexityFromString parses a label string into a Complexity.
// Returns Medium and false when the label is unknown.
//
// Tier labels (SMALL, MID, FRONTIER; used by tier-labelled splits) are accepted
// too and map to a complexity of the same tier (SIMPLE, MEDIUM, COMPLEX), so
// every tier metric is exact. For those rows the complexity "match" is a
// tier match (see isTierLabel), and in the confusion matrix SMALL rows
// appear under SIMPLE.
func complexityFromString(s string) (core.Complexity, bool) {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "TRIVIAL":
		return core.Trivial, true
	case "SIMPLE", "SMALL":
		return core.Simple, true
	case "MEDIUM", "MID":
		return core.Medium, true
	case "COMPLEX", "FRONTIER":
		return core.Complex, true
	default:
		return core.Medium, false
	}
}

// isTierLabel reports a tier label (SMALL, MID, FRONTIER) rather than a
// complexity label.
func isTierLabel(s string) bool {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "SMALL", "MID", "FRONTIER":
		return true
	}
	return false
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
		correct := cls.Complexity == expected
		if isTierLabel(t.Label) {
			correct = core.TierOf(cls.Complexity) == core.TierOf(expected)
		}
		results = append(results, Result{
			Task:      t,
			Predicted: cls.Complexity,
			Correct:   correct,
			Confident: cls.Confident,
			NoSignal:  cls.NoSignal,
		})
	}
	if skipped > 0 {
		fmt.Fprintf(w, "skipped %d tasks with unrecognised labels\n\n", skipped)
	}
	return results
}
