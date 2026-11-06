// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

// An agent that declares a default tier (agents/*.md) and is spawned with no
// model of its own runs on that tier instead of the inherited model. Tested at
// the shared decider for every harness: each one computes and records the
// decision, whether or not its protocol can write it.
func TestAgentDefaultTier_SharedDecider(t *testing.T) {
	cases := []struct {
		name      string
		agentType string
		inherited bool   // the call names no model
		prompt    string // may carry a tier hint or kind
		policy    string // project policy.json; "" writes none
		wantTier  core.Tier
		wantMoved bool
	}{
		{"unnamed model: mechanical-worker runs small", "downshift:mechanical-worker", true, "plain task", "", core.TierSmall, true},
		{"unnamed model: coder runs mid", "downshift:coder", true, "plain task", "", core.TierMid, true},
		{"unnamed model: architect keeps the inherited frontier", "downshift:architect", true, "plain task", "", core.TierFrontier, false},
		{"named model wins (O8)", "downshift:mechanical-worker", false, "plain task", "", core.TierFrontier, false},
		{"unknown agent type is today's behaviour", "downshift:no-such-agent", true, "plain task", "", core.TierFrontier, false},
		{"a bare name is not the squad's agent", "mechanical-worker", true, "plain task", "", core.TierFrontier, false},
		{"no agent type is today's behaviour", "", true, "plain task", "", core.TierFrontier, false},
		{"a hint only moves down: small hint under a mid default", "downshift:coder", true, "[tier=small] plain task", "", core.TierSmall, true},
		{"a hint never lands above the default", "downshift:mechanical-worker", true, "[tier=mid] plain task", "", core.TierSmall, true},
		{"a declared rule wins over the default", "downshift:mechanical-worker", true, "[kind=docs] plain task",
			`{"version":1,"routing":{"rules":[{"id":"docs","kinds":["docs"],"tier":"mid"}]}}`, core.TierMid, true},
		{"the project tier floor raises the default", "downshift:mechanical-worker", true, "plain task",
			`{"version":1,"min_tier":"mid"}`, core.TierMid, true},
		{"an invalid policy keeps (doubt keeps)", "downshift:mechanical-worker", true, "plain task",
			`{"version":1,"routing":{"rules":[{"id":"x"}]}}`, core.TierFrontier, false},
	}
	for _, n := range core.AllHarnessCaps() {
		frontier := cmdCat.ModelFor(n.Harness, core.TierFrontier)
		if frontier.ID == "" {
			continue // a harness with no catalog models routes nothing
		}
		prefix := n.Caps.SquadAgentPrefix
		for _, tc := range cases {
			agentType := tc.agentType
			if rest, ok := strings.CutPrefix(agentType, "downshift:"); ok {
				agentType = prefix + rest // each harness spells the squad's type its own way
			}
			if agentType == "mechanical-worker" && prefix == "" {
				continue // with no prefix a bare name is the squad's agent
			}
			t.Run(n.Harness+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
				t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
				if tc.policy != "" {
					writeFile(t, filepath.Join(dir, ".downshift", "policy.json"), tc.policy)
				}
				t.Cleanup(hookctx.SetProjectDir(dir))

				d := routingDecider{now: func() time.Time { return deciderNow }}.Decide(core.Request{
					Prompt: tc.prompt, Harness: n.Harness, AgentType: agentType,
					ModelInherited: tc.inherited, CurrentModelID: frontier.ID, Resolver: cmdCat,
				})
				moved := d.Verdict == core.VerdictDownshift || d.Verdict == core.VerdictUpshift
				gotTier := frontier.Tier
				if moved {
					gotTier = d.Model.Tier
				}
				if moved != tc.wantMoved || gotTier != tc.wantTier {
					t.Errorf("moved=%v tier=%v (verdict %v), want moved=%v tier=%v", moved, gotTier, d.Verdict, tc.wantMoved, tc.wantTier)
				}
				recorded := slices.Contains(fromcore.ReasonCodes(d), telemetry.ReasonAgentDefaultTier)
				wantRecorded := tc.wantMoved && d.Note.Verdict == "agent_default"
				if recorded != wantRecorded {
					t.Errorf("reason AGENT_DEFAULT_TIER recorded = %v, want %v (note verdict %q)", recorded, wantRecorded, d.Note.Verdict)
				}
			})
		}
	}
}
