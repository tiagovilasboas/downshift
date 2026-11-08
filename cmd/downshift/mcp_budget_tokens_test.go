// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
)

func TestMcpBudgetTokensPrecedence(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	root := t.TempDir()
	path := policy.ProjectPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"orchestration":{"budget_tokens":4000}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	env := func(v string) func(string) string { return func(string) string { return v } }
	cases := []struct {
		name string
		root string
		env  string
		want int64
	}{
		{"file", root, "", 4000},
		{"env wins", root, "250", 250},
		{"bad env falls back to the file", root, "lots", 4000},
		{"nothing set", t.TempDir(), "", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mcpBudgetTokens(c.root, env(c.env))(); got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}
