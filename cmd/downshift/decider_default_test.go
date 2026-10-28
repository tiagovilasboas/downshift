// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
)

// TestRoutingDeciderDefaultProfile runs the built-in agent types through the
// production decider. The default table moves them down only when the project
// policy loads; an unusable policy drops the agent type, so the spawn is kept.
func TestRoutingDeciderDefaultProfile(t *testing.T) {
	cases := []struct {
		name      string
		policy    string // project policy.json; "" writes none
		agentType string
		wantModel string
	}{
		{"no policy: Explore goes small", "", "Explore", smallID},
		{"no policy: general-purpose keeps", "", "general-purpose", frontierID},
		{"invalid policy: Explore is kept", `{"version":1,"routing":{"rules":[{"id":"x"}]}}`, "Explore", frontierID},
		{"malformed policy: Explore is kept", `{not json`, "Explore", frontierID},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
			t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
			if tc.policy != "" {
				writeFile(t, filepath.Join(dir, ".downshift", "policy.json"), tc.policy)
			}
			t.Cleanup(hookctx.SetProjectDir(dir))

			d := routingDecider{now: func() time.Time { return deciderNow }}.Decide(core.Request{
				Prompt: "plain task", Harness: "claude-code", AgentType: tc.agentType,
				CurrentModelID: frontierID, Resolver: cmdCat,
			})
			got := d.Model.ID
			if d.Verdict != core.VerdictDownshift && d.Verdict != core.VerdictUpshift {
				got = frontierID // not a move: the spawn runs on the requested model
			}
			if got != tc.wantModel {
				t.Errorf("model = %q (verdict %v), want %q", got, d.Verdict, tc.wantModel)
			}
		})
	}
}
