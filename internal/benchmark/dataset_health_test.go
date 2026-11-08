// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/benchmark"
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

func TestLoadDataset_SampleHasNoDuplicates(t *testing.T) {
	tasks, err := benchmark.LoadDataset("testdata/sample.json")
	if err != nil {
		t.Fatalf("sample dataset must load cleanly: %v", err)
	}
	if len(tasks) != 8 {
		t.Errorf("sample len = %d, want 8", len(tasks))
	}
}
