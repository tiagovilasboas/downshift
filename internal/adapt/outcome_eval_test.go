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
	if a.Total != 40 || a.BaselinePass != 38 || a.AdjustedPass != 40 || a.Down != 25 || a.Up != 2 || a.Same != 13 {
		t.Fatalf("A: got %+v", a)
	}
	b, err := EvalSharedShapeMemory(tasks, runs)
	if err != nil {
		t.Fatal(err)
	}
	if b.Total != 40 || b.BaselinePass != 38 || b.AdjustedPass != 30 || b.Down != 32 || b.Up != 0 || b.Same != 8 {
		t.Fatalf("B: got %+v", b)
	}
}
