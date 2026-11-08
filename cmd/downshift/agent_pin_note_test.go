// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/hookctx"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry/fromcore"
)

// A spawn that names a model above its agent's default tier is told so, and
// nothing moves. Tested at the shared decider for every harness: the reason
// code is recorded everywhere, the note is set only where the harness declares
// CanReturnNote.
func TestAgentPinNote_SharedDecider(t *testing.T) {
	for _, n := range core.AllHarnessCaps() {
		frontier := cmdCat.ModelFor(n.Harness, core.TierFrontier)
		if frontier.ID == "" {
			continue
		}
		for _, tc := range []struct {
			name      string
			agent     string
			inherited bool
			want      bool
		}{
			{"mid agent pinned to frontier", "downshift:coder", false, true},
			{"agent pinned at its default", "downshift:architect", false, false},
			{"inherited model is not a pin", "downshift:coder", true, false},
			{"not a squad agent", "other:coder", false, false},
		} {
			agent := tc.agent
			if rest, ok := strings.CutPrefix(agent, "downshift:"); ok {
				agent = n.Caps.SquadAgentPrefix + rest
			}
			t.Run(n.Harness+"/"+tc.name, func(t *testing.T) {
				t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
				t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
				t.Cleanup(hookctx.SetProjectDir(t.TempDir()))
				d := routingDecider{now: func() time.Time { return deciderNow }}.Decide(core.Request{
					Prompt: "plain task", Harness: n.Harness, AgentType: agent,
					ModelInherited: tc.inherited, CurrentModelID: frontier.ID, Resolver: cmdCat,
				})
				if d.Verdict == core.VerdictDownshift || d.Verdict == core.VerdictUpshift {
					if !tc.inherited {
						t.Fatalf("a pinned model moved (verdict %v)", d.Verdict)
					}
					return
				}
				if got := slices.Contains(fromcore.ReasonCodes(d), telemetry.ReasonAboveAgentDefault); got != tc.want {
					t.Errorf("ABOVE_AGENT_DEFAULT recorded = %v, want %v", got, tc.want)
				}
				wantNote := tc.want && n.Caps.CanReturnNote
				if got := d.Advice == core.AdviceAboveAgentDefault; got != wantNote {
					t.Errorf("note = %v, want %v (CanReturnNote %v)", got, wantNote, n.Caps.CanReturnNote)
				}
				if wantNote && core.AgentPinNote(d.AgentPin) != "downshift: coder defaults to mid; you pinned frontier. It runs as pinned." {
					t.Errorf("note text = %q", core.AgentPinNote(d.AgentPin))
				}
			})
		}
	}
}
