// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/outcome"
)

func TestEvalContrafactualPerTask_LabsFixture(t *testing.T) {
	root := filepath.Join(os.Getenv("HOME"), "Github", "downshift-labs", "eval", "outcomes")
	if _, err := os.Stat(filepath.Join(root, "runs", "small", "run.json")); err != nil {
		t.Skip("downshift-labs outcome runs not present")
	}
	tasks, err := outcome.Load(filepath.Join(root, "testdata"))
	if err != nil {
		t.Fatal(err)
	}
	runs, err := outcome.LoadRuns(filepath.Join(root, "runs"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := EvalContrafactualPerTask(tasks, runs)
	if err != nil {
		t.Fatal(err)
	}
	// Pinned after the semantic boost stopped lifting no-signal prompts: eight
	// small tasks (fizzbuzz, dedupe, chunk, slugify, anagram, parse-kv,
	// merge-intervals, word-wrap) used to land on the frontier tier and now land
	// on mid. Baseline pass moved 38 -> 37 in the recorded runs.
	if a.Total != 40 || a.BaselinePass != 37 || a.AdjustedPass != 40 || a.Down != 23 || a.Up != 3 || a.Same != 14 {
		t.Fatalf("A: got %+v", a)
	}
	b, err := EvalSharedShapeMemory(tasks, runs)
	if err != nil {
		t.Fatal(err)
	}
	if b.Total != 40 || b.BaselinePass != 37 || b.AdjustedPass != 37 || b.Down != 11 || b.Up != 0 || b.Same != 29 {
		t.Fatalf("B: got %+v", b)
	}
	// The floor was 38/40 before the semantic boost stopped lifting no-signal
	// prompts to the frontier tier. It is 37/40 on the recorded runs: one small
	// task passed only on the frontier tier. This is an explicit cost-vs-quality
	// choice of that change, not a drift; do not lower it further.
	if b.AdjustedPass < 37 {
		t.Fatalf("shared memory gate: adjusted pass %d/40 below classifier floor 37/40", b.AdjustedPass)
	}
}
