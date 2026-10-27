// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/quota"
)

var deciderNow = time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

const (
	frontierID = "claude-opus-4-5"
	smallID    = "claude-haiku-4-5"
	gatedRule  = `{"version":1,"routing":{"rules":[{"id":"cheap","tier":"small",
		"gates":[{"harness":"claude-code","window":"short","below":50}]}]}}`
)

// snapshot is a claude-code statusline reading of the short window, observed
// age ago with a five minute life.
func snapshot(age time.Duration, used float64) quota.Snapshot {
	observed := deciderNow.Add(-age)
	return quota.Snapshot{
		Version: 1, Harness: "claude-code", Source: "claude-statusline",
		ObservedAt: observed, ExpiresAt: observed.Add(5 * time.Minute),
		Windows: []quota.Window{{ID: "five_hour", Scope: "harness", UsedPercent: &used}},
	}
}

// TestRoutingDeciderMovesOnlyWhenV2Decides drives the production decider with
// a temporary project directory: it changes the spawn only when routing v2
// decides to, and leaves it unchanged on every error.
func TestRoutingDeciderMovesOnlyWhenV2Decides(t *testing.T) {
	cases := []struct {
		name      string
		policy    string // project policy.json; "" writes none
		quota     *quota.Snapshot
		prompt    string
		model     string
		wantModel string
	}{
		{"no policy and no hint keeps", "", nil, "plain task", frontierID, frontierID},
		{"no policy: a hint below the request moves", "", nil, "[tier=small] plain task", frontierID, smallID},
		{"valid rule moves", gatedRule, nil, "plain task", frontierID, smallID},
		{"valid rule gate fails on a fresh hot reading", gatedRule, ptr(snapshot(time.Minute, 99)), "plain task", frontierID, frontierID},
		{"valid rule gate passes on a fresh cool reading", gatedRule, ptr(snapshot(time.Minute, 10)), "plain task", frontierID, smallID},
		{"expired hot reading is unknown and passes", gatedRule, ptr(snapshot(10*time.Minute, 99)), "plain task", frontierID, smallID},
		{"floor lifts a small request", `{"version":1,"min_tier":"frontier"}`, nil, "plain task", smallID, cmdCat.ModelFor("claude-code", core.TierFrontier).ID},
		{"invalid policy keeps despite a hint", `{"version":1,"routing":{"rules":[{"id":"x"}]}}`, nil, "[tier=small] plain task", frontierID, frontierID},
		{"malformed policy keeps despite a hint", `{not json`, nil, "[tier=small] plain task", frontierID, frontierID},
		{"invalid floor keeps despite a hint", `{"version":1,"min_tier":"huge"}`, nil, "[tier=small] plain task", frontierID, frontierID},
		{"unreadable policy keeps despite a hint", strings.Repeat(" ", 70<<10), nil, "[tier=small] plain task", frontierID, frontierID},
		{"unknown requested model keeps", gatedRule, nil, "plain task", "no-such-model", "no-such-model"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
			if tc.policy != "" {
				writeFile(t, filepath.Join(dir, ".downshift", "policy.json"), tc.policy)
			}
			if tc.quota != nil {
				qp, _ := quota.Path()
				if err := quota.Store(qp, *tc.quota); err != nil {
					t.Fatal(err)
				}
			}
			t.Cleanup(hookctx.SetProjectDir(dir))

			d := routingDecider{now: func() time.Time { return deciderNow }}.Decide(core.Request{
				Prompt: tc.prompt, Harness: "claude-code", CurrentModelID: tc.model, Resolver: cmdCat,
			})
			got := d.Model.ID
			if d.Verdict != core.VerdictDownshift && d.Verdict != core.VerdictUpshift {
				got = tc.model // not a move: the spawn runs on the requested model
			}
			if got != tc.wantModel {
				t.Errorf("model = %q (verdict %v), want %q", got, d.Verdict, tc.wantModel)
			}
		})
	}
}

// TestRoutingDeciderNoCatalogKeeps pins the fail-open path with no resolver.
func TestRoutingDeciderNoCatalogKeeps(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	d := routingDecider{now: time.Now}.Decide(core.Request{Prompt: "[tier=small] x", Harness: "claude-code", CurrentModelID: frontierID})
	if d.Verdict == core.VerdictDownshift || d.Model.ID != "" {
		t.Errorf("no catalog: got verdict %v model %q, want no move", d.Verdict, d.Model.ID)
	}
}

// TestRoutingDeciderUnknownModelIsNotPlanned pins the hand-off to channel
// planning: a spawn whose model is absent stays kept, so a known session list
// gives the plan nothing to write.
func TestRoutingDeciderUnknownModelIsNotPlanned(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Cleanup(hookctx.SetProjectDir(t.TempDir()))
	d := routingDecider{now: time.Now}.Decide(core.Request{Prompt: "[tier=small] plain task", Harness: "claude-code", Resolver: cmdCat})
	session := core.KnownSession([]string{smallID, frontierID})
	if plan := d.PlanSession(core.ClaudeCodeCaps, cmdCat, session, ""); plan.RewriteModel {
		t.Fatalf("a spawn with no model was planned for a rewrite: %+v", plan)
	}
}

func ptr[T any](v T) *T { return &v }
