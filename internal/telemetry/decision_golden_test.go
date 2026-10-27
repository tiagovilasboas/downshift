// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

var updateGolden = flag.Bool("update-decision-golden", false, "rewrite testdata/decision_events.golden.jsonl")

// goldenDecisions are the decisions whose events the golden file pins: every
// prompt of the core corpus routed on a known frontier baseline and on an
// unknown one, plus hand-built decisions for the shadow and held fields the
// corpus does not reach.
func goldenDecisions(t *testing.T) []core.Decision {
	t.Helper()
	t.Setenv(core.NBUpshiftEnv, "")
	t.Setenv(core.NBDownshiftEnv, "")
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", "")
	cat := catalog.Load()
	files, err := filepath.Glob(filepath.Join("..", "core", "testdata", "corpus", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("corpus files: %v (%d)", err, len(files))
	}
	sort.Strings(files)
	var out []core.Decision
	frontier := cat.ModelFor("claude-code", core.TierFrontier).ID
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var cases []struct {
			Prompt string `json:"prompt"`
		}
		if err := json.Unmarshal(raw, &cases); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, c := range cases {
			out = append(out, core.Route(c.Prompt, "claude-code", frontier, cat))
			out = append(out, core.Route(c.Prompt, "codex", "", cat))
		}
	}
	return append(out, handBuiltDecisions()...)
}

func handBuiltDecisions() []core.Decision {
	return []core.Decision{
		{},
		{Harness: "cursor", Guards: core.Guards{PolicyFloor: true, RiskFloor: true, SemanticLift: true, TaskFocus: true, AgentTypeHint: true}},
		{
			Harness: "claude-code", Complexity: core.Complex, Tier: core.TierFrontier, Verdict: core.VerdictUpshift, Confident: true,
			Model:    core.Model{ID: "big", InputM: 15, OutputM: 75},
			Baseline: core.Baseline{CurrentModel: core.Model{ID: "mid", InputM: 3, OutputM: 15}, FromSettings: true, GuessHeld: true, Pinned: true},
			Session:  core.Session{QuotaStatus: "ok", QuotaSource: "payload", CreditHeld: true},
			Savings:  -0.25,
		},
		{
			Harness: "codex", Complexity: core.Medium, Tier: core.TierMid, Verdict: core.VerdictDownshift, SafeVerdict: core.VerdictOK,
			Corrections: []string{core.RuleUnconfidentDowngrade, core.RuleRiskDowngrade, "R99_FUTURE"},
			Risk:        []string{"auth"}, NoSignal: true, TaskNotVisible: true, UnsupportedLanguage: true,
		},
		{
			Harness: "opencode", Tier: core.TierMid, Verdict: core.VerdictOK, Model: core.Model{ID: "m-70"},
			Shadow: core.Shadow{
				ScoreWould: "m-80", ScoreDiffers: true, ScoreWouldCostRatio: 1.234567, ScoreApplied: true,
				GatewayWould: "deepseek/deepseek-v4.1-flash",
				NBUpshift:    core.NBUpshift{Would: true, From: core.TierMid, To: core.TierFrontier, Margin: 0.123456, Applied: true},
				NBDownshift:  core.NBDownshift{Would: true, Margin: 0.31234, Applied: true},
			},
		},
	}
}

// TestDecisionEventsGolden pins the serialized event of each golden decision
// byte for byte, so moving the Decision-to-Event mapping cannot change the log.
func TestDecisionEventsGolden(t *testing.T) {
	var buf bytes.Buffer
	for _, d := range goldenDecisions(t) {
		ev := telemetry.FromDecision(d, "0123456789abcdef", "test")
		ev.Timestamp = ""
		line, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	path := filepath.Join("testdata", "decision_events.golden.jsonl")
	if *updateGolden {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatalf("serialized decision events differ from %s", path)
	}
}
