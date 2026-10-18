// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestBenchmarkSplitsMatchClsAfterBaseline(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	benchDir := filepath.Join(home, ".downshift-eval", "opus-data", "bench")
	afterPath := filepath.Join(benchDir, "cls-after.jsonl")
	allPath := filepath.Join(benchDir, "all.jsonl")
	if _, err := os.Stat(afterPath); err != nil {
		t.Skip("no local cls-after baseline")
	}

	after := map[string]map[string]any{}
	f, err := os.Open(afterPath)
	if err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var row map[string]any
		if json.Unmarshal(sc.Bytes(), &row) != nil {
			continue
		}
		id, _ := row["ID"].(string)
		after[id] = row
	}

	cat := catalog.Load()
	resolver := cat
	frontier := resolver.ModelFor("claude-code", core.TierFrontier)
	if frontier.ID == "" {
		t.Fatal("no frontier model")
	}

	prompts, err := os.Open(allPath)
	if err != nil {
		t.Fatal(err)
	}
	ps := bufio.NewScanner(prompts)
	var diffs int
	for ps.Scan() {
		var item struct {
			ID     string `json:"id"`
			Prompt string `json:"prompt"`
		}
		if json.Unmarshal(ps.Bytes(), &item) != nil {
			continue
		}
		base := after[item.ID]
		if base == nil {
			t.Fatalf("missing baseline row %s", item.ID)
		}
		d := core.Route(item.Prompt, "claude-code", frontier.ID, resolver)
		wantC := strings.ToUpper(anyString(base["Complexity"]))
		if d.Complexity.String() != wantC {
			diffs++
			t.Errorf("%s complexity %s != %s", item.ID, d.Complexity, wantC)
			continue
		}
		wantConf, _ := base["Confident"].(bool)
		if d.Confident != wantConf {
			diffs++
			t.Errorf("%s confident %v != %v", item.ID, d.Confident, wantConf)
		}
	}
	if diffs > 0 {
		t.Fatalf("%d benchmark rows changed vs cls-after", diffs)
	}
}

func anyString(v any) string {
	s, _ := v.(string)
	return s
}
