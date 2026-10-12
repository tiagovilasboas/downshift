// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// TestPromptReport scores a directory of prompt files through core.Route and
// writes a JSON report: one row per prompt with the classification, tier,
// effort and whether the model would be written when the current model is
// unknown. It is a measurement aid, not an assertion, so it only runs when
// DOWNSHIFT_PROMPT_REPORT_IN (a directory of *.json arrays with id, prompt and
// optional dev_difficulty) and DOWNSHIFT_PROMPT_REPORT_OUT (a file) are set.
func TestPromptReport(t *testing.T) {
	in, out := os.Getenv("DOWNSHIFT_PROMPT_REPORT_IN"), os.Getenv("DOWNSHIFT_PROMPT_REPORT_OUT")
	if in == "" || out == "" {
		t.Skip("set DOWNSHIFT_PROMPT_REPORT_IN and DOWNSHIFT_PROMPT_REPORT_OUT to run")
	}
	files, _ := filepath.Glob(filepath.Join(in, "*.json"))
	sort.Strings(files)
	type row struct {
		ID            string   `json:"id"`
		File          string   `json:"file"`
		Prompt        string   `json:"prompt"`
		DevDifficulty string   `json:"dev_difficulty,omitempty"`
		Complexity    string   `json:"complexity"`
		Tier          string   `json:"tier"`
		Effort        string   `json:"effort"`
		Confident     bool     `json:"confident"`
		Written       bool     `json:"written_when_model_unknown"`
		HeldBy        []string `json:"held_by,omitempty"`
		RiskFloor     bool     `json:"risk_floor,omitempty"`
		GraphEsc      bool     `json:"graph_escalated,omitempty"`
		Chars         int      `json:"prompt_chars"`
	}
	var rows []row
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var in []struct {
			ID, Prompt, DevDifficulty string
		}
		var raw []map[string]any
		if err := json.Unmarshal(b, &raw); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, m := range raw {
			s := func(k string) string { v, _ := m[k].(string); return v }
			in = append(in, struct{ ID, Prompt, DevDifficulty string }{s("id"), s("prompt"), s("dev_difficulty")})
		}
		for _, c := range in {
			d := core.Route(c.Prompt, "claude-code", "", cat)
			rows = append(rows, row{
				ID: c.ID, File: filepath.Base(f), Prompt: c.Prompt, DevDifficulty: c.DevDifficulty,
				Complexity: d.Complexity.String(), Tier: d.Tier.String(), Effort: d.Effort.String(),
				Confident: d.Confident, Written: d.ShouldRewriteModel(), HeldBy: d.Corrections,
				RiskFloor: d.RiskFloor, GraphEsc: d.GraphEscalated, Chars: len(c.Prompt),
			})
		}
	}
	b, _ := json.MarshalIndent(rows, "", " ")
	if err := os.WriteFile(out, b, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d rows to %s", len(rows), out)
}
