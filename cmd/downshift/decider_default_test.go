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
		prompt    string // "" is a plain task the classifier is unsure of
		wantModel string
	}{
		{"no policy: Explore goes small", "", "Explore", "", smallID},
		{"no policy: general-purpose keeps", "", "general-purpose", "", frontierID},
		// The read-only review that ran on the frontier model on 2026-10-28.
		{"no policy: a routine general-purpose review goes mid", "", "general-purpose", reviewPrompt, midID()},
		{"no policy: Plan keeps a routine review", "", "Plan", reviewPrompt, frontierID},
		{"invalid policy: Explore is kept", `{"version":1,"routing":{"rules":[{"id":"x"}]}}`, "Explore", "", frontierID},
		{"malformed policy: Explore is kept", `{not json`, "Explore", "", frontierID},
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

			prompt := tc.prompt
			if prompt == "" {
				prompt = "plain task"
			}
			d := routingDecider{now: func() time.Time { return deciderNow }}.Decide(core.Request{
				Prompt: prompt, Harness: "claude-code", AgentType: tc.agentType,
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

// reviewPrompt is a read-only documentation review brief handed to a
// general-purpose subagent: read files, grade them, change nothing. It is the
// shape of the reviews that ran on the frontier model on 2026-10-28.
const reviewPrompt = "First read your role file: agents/senior-staff-qa.md. You are reviewing documentation of the repo. " +
	"READ-ONLY: do not edit, create, move or delete any file; no state-changing git; no network. " +
	"Read-only commands (grep, ls, find, a small script that only reads) are fine. " +
	"Your scope (verifiability): docs/evidence/**, docs/spikes/**, docs/releases/**, CHANGELOG.md. " +
	"Also check relative links: count total relative links and how many point to a file that does not exist. " +
	"Output (max 350 words): a grade 0-10 with one sentence of justification; 4-6 findings with file:line evidence; " +
	"top 3 fixes in priority order."

func midID() string { return cmdCat.ModelFor("claude-code", core.TierMid).ID }
