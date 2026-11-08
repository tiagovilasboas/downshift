// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

// belowFixtureEnv points the report at testdata/report_below_session: a
// claude-code log with one spawn below its opus session, one at the session
// tier and one with no parent model, and a codex log with no parent model.
func belowFixtureEnv(t *testing.T) {
	t.Helper()
	endEnv(t)
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	raw, err := os.ReadFile(filepath.Join("testdata", "report_below_session", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(log, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
}

func TestReport_BelowSessionModelText(t *testing.T) {
	belowFixtureEnv(t)
	tab := runReportOut(t, "--days=0", "--text")
	// Billable: s1 = 1000+500+200 write (cache read 8800 excluded) = 1.7k below;
	// s2 = 500 at the session tier. 2.2k observed, 1 of 2 spawns; s3 has no
	// parent model and is placed against its requested model apart.
	for _, want := range []string{
		"ran below the session model  1.7k of 2.2k billable (77%; 1 of 2 spawns)",
		"below the requested model",
		"cache read  8.8k (apart from billable)",
	} {
		if !strings.Contains(tab, want) {
			t.Errorf("routing section lacks %q:\n%s", want, tab)
		}
	}
	// The below-session line comes first within a harness, and no total row exists.
	if strings.Index(tab, "ran below the session model") > strings.Index(tab, "below the requested model") {
		t.Errorf("below-session row must come first:\n%s", tab)
	}
	if !strings.Contains(tab, "unobserved  1 spawn (no model could be ranked)") {
		t.Errorf("codex has no rankable model and must read unobserved:\n%s", tab)
	}
}

func TestReport_BelowSessionModelJSON(t *testing.T) {
	belowFixtureEnv(t)
	var rep struct {
		Tokens []struct {
			Harness  string `json:"harness"`
			Observed bool   `json:"routing_observed"`
			Below    struct {
				Spawns           int   `json:"spawns"`
				BelowSpawns      int   `json:"below_spawns"`
				FallbackSpawns   int   `json:"fallback_spawns"`
				UnobservedSpawns int   `json:"unobserved_spawns"`
				Billable         int64 `json:"billable"`
				BelowBillable    int64 `json:"below_billable"`
			} `json:"routing"`
		} `json:"tokens_by_harness"`
	}
	if err := json.Unmarshal([]byte(runReportOut(t, "--days=0", "--json")), &rep); err != nil {
		t.Fatal(err)
	}
	for _, r := range rep.Tokens {
		switch r.Harness {
		case "claude-code":
			if !r.Observed || r.Below.BelowBillable != 1700 || r.Below.Billable != 2200 || r.Below.FallbackSpawns != 1 {
				t.Errorf("claude-code = %+v", r)
			}
		case "codex":
			if r.Observed || r.Below.BelowBillable != 0 || r.Below.Spawns != 0 || r.Below.UnobservedSpawns != 1 {
				t.Errorf("codex = %+v", r)
			}
		}
	}
}
