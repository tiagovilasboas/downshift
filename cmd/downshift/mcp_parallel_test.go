// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

func TestMcpParallelForEveryHarness(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".downshift"), 0o755); err != nil {
		t.Fatal(err)
	}
	pf := filepath.Join(root, ".downshift", "policy.json")
	if err := os.WriteFile(pf, []byte(`{"version":1,"orchestration":{"max_parallel":8}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	none := func(string) string { return "" }
	for _, n := range core.AllHarnessCaps() {
		c := mcpParallel(root, none)(n.Harness)
		want, src := 8, "policy"
		if n.Caps.MaxParallel > 0 {
			want, src = n.Caps.MaxParallel, "harness"
		}
		if c.Max != want || c.Source != src || c.WhenStrained < 1 {
			t.Errorf("%s: %+v", n.Harness, c)
		}
	}
	env := func(string) string { return "2" }
	if c := mcpParallel(root, env)("codex"); c.Max != 2 || c.Source != "user" || c.WhenStrained != 1 {
		t.Errorf("user override: %+v", c)
	}
}
