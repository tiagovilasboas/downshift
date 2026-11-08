// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package decide

import (
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
)

// A model pinned above the agent's default tier is flagged only when it stayed
// as pinned: a rule that moved the spawn means "nothing moved" would be false.
// The note for the orchestrator needs a harness that can return one.
func TestDecideAboveAgentDefault(t *testing.T) {
	build := func(harness contract.HarnessID, mod func(*contract.Request)) Input {
		return Input{
			Request: req(func(r *contract.Request) {
				r.Harness = harness
				r.AgentType = "downshift:coder"
				if mod != nil {
					mod(r)
				}
			}),
			Policy: testPolicy(), Now: now, Catalog: newCatalog(),
			AgentDefaultTier: contract.TierMid, AgentName: "coder",
		}
	}
	d, _ := Decide(build("claude-code", nil))
	if !d.Guards.AboveAgentDefault || d.Advice != core.AdviceAboveAgentDefault || d.AgentPin.Agent != "coder" {
		t.Fatalf("kept: guards=%v advice=%q pin=%+v", d.Guards.AboveAgentDefault, d.Advice, d.AgentPin)
	}
	d, _ = Decide(build(h, nil))
	if !d.Guards.AboveAgentDefault || d.Advice != "" {
		t.Fatalf("harness without notes: guards=%v advice=%q, want code only", d.Guards.AboveAgentDefault, d.Advice)
	}
	moved, n := Decide(build("claude-code", func(r *contract.Request) { r.Hint.Kind = contract.KindDocs }))
	if n.TierTo == n.TierFrom {
		t.Fatalf("fixture did not move the spawn (verdict %q)", n.Verdict)
	}
	if moved.Guards.AboveAgentDefault || moved.Advice != "" {
		t.Fatalf("moved spawn: guards=%v advice=%q, want none", moved.Guards.AboveAgentDefault, moved.Advice)
	}
}
