// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/benchmark"
)

// --- LoadDataset duplicate guard (table-driven) ---

func TestLoadDataset_Duplicates(t *testing.T) {
	cases := []struct {
		name    string
		tasks   []benchmark.Task
		wantErr string // "" means no error expected
	}{
		{
			name: "no duplicates loads cleanly",
			tasks: []benchmark.Task{
				{Prompt: "rename the variable", Label: "TRIVIAL"},
				{Prompt: "rearchitect the auth system", Label: "COMPLEX"},
			},
			wantErr: "",
		},
		{
			name: "exact duplicate rejected",
			tasks: []benchmark.Task{
				{Prompt: "rename the variable", Label: "TRIVIAL"},
				{Prompt: "rename the variable", Label: "TRIVIAL"},
			},
			wantErr: "duplicate prompt at index 1",
		},
		{
			name: "case-insensitive duplicate rejected",
			tasks: []benchmark.Task{
				{Prompt: "Rename The Variable", Label: "TRIVIAL"},
				{Prompt: "rename the variable", Label: "SIMPLE"},
			},
			wantErr: "duplicate prompt at index 1",
		},
		{
			name: "whitespace-only difference rejected",
			tasks: []benchmark.Task{
				{Prompt: "  rename the variable  ", Label: "TRIVIAL"},
				{Prompt: "rename the variable", Label: "TRIVIAL"},
			},
			wantErr: "duplicate prompt at index 1",
		},
		{
			name: "later duplicate names its own index",
			tasks: []benchmark.Task{
				{Prompt: "task a", Label: "TRIVIAL"},
				{Prompt: "task b", Label: "SIMPLE"},
				{Prompt: "TASK A", Label: "MEDIUM"},
			},
			wantErr: "duplicate prompt at index 2",
		},
		{
			name: "empty prompts are not duplicates",
			tasks: []benchmark.Task{
				{Prompt: "", Label: "TRIVIAL"},
				{Prompt: "   ", Label: "SIMPLE"},
			},
			wantErr: "",
		},
		{
			name:    "empty dataset loads cleanly",
			tasks:   []benchmark.Task{},
			wantErr: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeDataset(t, tc.tasks)
			_, err := benchmark.LoadDataset(path)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestLoadDataset_SeedHasNoDuplicates(t *testing.T) {
	tasks, err := benchmark.LoadDataset("../../benchmark/tasks.json")
	if err != nil {
		t.Fatalf("seed dataset must still load cleanly: %v", err)
	}
	if len(tasks) != 200 {
		t.Errorf("seed len = %d, want 200", len(tasks))
	}
}

// --- DatasetHealth (table-driven) ---

func TestDatasetHealth(t *testing.T) {
	cases := []struct {
		name          string
		tasks         []benchmark.Task
		wantLabels    map[string]int
		wantUnknown   int
		wantEmpty     int
		wantDuplicate int
	}{
		{
			name:          "empty dataset",
			tasks:         nil,
			wantLabels:    map[string]int{"TRIVIAL": 0, "SIMPLE": 0, "MEDIUM": 0, "COMPLEX": 0},
			wantUnknown:   0,
			wantEmpty:     0,
			wantDuplicate: 0,
		},
		{
			name: "balanced labels",
			tasks: []benchmark.Task{
				{Prompt: "a", Label: "TRIVIAL"},
				{Prompt: "b", Label: "simple"},
				{Prompt: "c", Label: " MEDIUM "},
				{Prompt: "d", Label: "COMPLEX"},
			},
			wantLabels:    map[string]int{"TRIVIAL": 1, "SIMPLE": 1, "MEDIUM": 1, "COMPLEX": 1},
			wantUnknown:   0,
			wantEmpty:     0,
			wantDuplicate: 0,
		},
		{
			name: "unknown labels counted separately",
			tasks: []benchmark.Task{
				{Prompt: "a", Label: "BANANA"},
				{Prompt: "b", Label: ""},
				{Prompt: "c", Label: "TRIVIAL"},
			},
			wantLabels:    map[string]int{"TRIVIAL": 1, "SIMPLE": 0, "MEDIUM": 0, "COMPLEX": 0},
			wantUnknown:   2,
			wantEmpty:     0,
			wantDuplicate: 0,
		},
		{
			name: "empty prompts counted",
			tasks: []benchmark.Task{
				{Prompt: "", Label: "TRIVIAL"},
				{Prompt: "   ", Label: "SIMPLE"},
				{Prompt: "real task", Label: "MEDIUM"},
			},
			wantLabels:    map[string]int{"TRIVIAL": 1, "SIMPLE": 1, "MEDIUM": 1, "COMPLEX": 0},
			wantUnknown:   0,
			wantEmpty:     2,
			wantDuplicate: 0,
		},
		{
			name: "case-insensitive duplicates counted once per repeat",
			tasks: []benchmark.Task{
				{Prompt: "Do the thing", Label: "TRIVIAL"},
				{Prompt: "do the thing", Label: "SIMPLE"},
				{Prompt: "  DO THE THING ", Label: "MEDIUM"},
				{Prompt: "other", Label: "COMPLEX"},
			},
			wantLabels:    map[string]int{"TRIVIAL": 1, "SIMPLE": 1, "MEDIUM": 1, "COMPLEX": 1},
			wantUnknown:   0,
			wantEmpty:     0,
			wantDuplicate: 2,
		},
		{
			name: "distinct prompts are not duplicates",
			tasks: []benchmark.Task{
				{Prompt: "task one", Label: "TRIVIAL"},
				{Prompt: "task two", Label: "TRIVIAL"},
			},
			wantLabels:    map[string]int{"TRIVIAL": 2, "SIMPLE": 0, "MEDIUM": 0, "COMPLEX": 0},
			wantUnknown:   0,
			wantEmpty:     0,
			wantDuplicate: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := benchmark.DatasetHealth(tc.tasks)
			if got.Total != len(tc.tasks) {
				t.Errorf("Total = %d, want %d", got.Total, len(tc.tasks))
			}
			for _, label := range []string{"TRIVIAL", "SIMPLE", "MEDIUM", "COMPLEX"} {
				if got.CountsByLabel[label] != tc.wantLabels[label] {
					t.Errorf("CountsByLabel[%q] = %d, want %d", label, got.CountsByLabel[label], tc.wantLabels[label])
				}
			}
			if got.Unknown != tc.wantUnknown {
				t.Errorf("Unknown = %d, want %d", got.Unknown, tc.wantUnknown)
			}
			if got.EmptyCount != tc.wantEmpty {
				t.Errorf("EmptyCount = %d, want %d", got.EmptyCount, tc.wantEmpty)
			}
			if got.DuplicateCount != tc.wantDuplicate {
				t.Errorf("DuplicateCount = %d, want %d", got.DuplicateCount, tc.wantDuplicate)
			}
		})
	}
}

func TestDatasetHealth_Seed(t *testing.T) {
	tasks, err := benchmark.LoadDataset("../../benchmark/tasks.json")
	if err != nil {
		t.Fatalf("cannot load seed dataset: %v", err)
	}
	got := benchmark.DatasetHealth(tasks)
	want := map[string]int{"TRIVIAL": 50, "SIMPLE": 50, "MEDIUM": 50, "COMPLEX": 50}
	for label, n := range want {
		if got.CountsByLabel[label] != n {
			t.Errorf("seed CountsByLabel[%q] = %d, want %d", label, got.CountsByLabel[label], n)
		}
	}
	if got.Unknown != 0 {
		t.Errorf("seed Unknown = %d, want 0", got.Unknown)
	}
	if got.EmptyCount != 0 {
		t.Errorf("seed EmptyCount = %d, want 0", got.EmptyCount)
	}
	if got.DuplicateCount != 0 {
		t.Errorf("seed DuplicateCount = %d, want 0", got.DuplicateCount)
	}
}
