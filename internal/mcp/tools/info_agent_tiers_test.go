// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/agenttier"
)

// providers and playbook name each squad agent's default tier (agent and tier
// only), so a harness that sends no agent type can still choose by it.
func TestAgentDefaultTiersInProvidersForEveryHarness(t *testing.T) {
	want := agenttier.List()
	if len(want) != 15 {
		t.Fatalf("agent table has %d entries, want 15", len(want))
	}
	d := fullDeps()
	d.Harnesses = func() []string {
		var ids []string
		for _, n := range core.AllHarnessCaps() {
			ids = append(ids, n.Harness)
		}
		return ids
	}
	for _, n := range core.AllHarnessCaps() {
		t.Run(n.Harness, func(t *testing.T) {
			env, err := call(t, d, "providers", `{"harness":"`+n.Harness+`"}`)
			if err != nil {
				t.Fatal(err)
			}
			var got providersData
			if err := json.Unmarshal(env["data"], &got); err != nil {
				t.Fatal(err)
			}
			if len(got.AgentDefaultTiers) != len(want) || got.AgentPrefix != n.Caps.SquadAgentPrefix {
				t.Fatalf("tiers=%d prefix=%q, want %d / %q", len(got.AgentDefaultTiers), got.AgentPrefix, len(want), n.Caps.SquadAgentPrefix)
			}
			for i, e := range got.AgentDefaultTiers {
				if e != want[i] || e.Tier == "" {
					t.Errorf("entry %d = %+v, want %+v", i, e, want[i])
				}
			}
		})
	}
}

func TestAgentDefaultTiersInPlaybookList(t *testing.T) {
	env, err := call(t, fullDeps(), "playbook", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var l playbookList
	if err := json.Unmarshal(env["data"], &l); err != nil {
		t.Fatal(err)
	}
	tiers := map[string]string{}
	for _, e := range l.AgentDefaultTiers {
		tiers[e.Agent] = e.Tier
	}
	if tiers["mechanical-worker"] != "small" || tiers["coder"] != "mid" || tiers["coder-back"] != "mid" || tiers["coder-front"] != "mid" || tiers["architect"] != "frontier" {
		t.Fatalf("agent tiers = %v", tiers)
	}
	var raw struct {
		Tiers []map[string]any `json:"agent_default_tiers"`
	}
	_ = json.Unmarshal(env["data"], &raw)
	for _, e := range raw.Tiers {
		for k := range e {
			if k != "agent" && k != "tier" && k != "playbook" {
				t.Errorf("entry %v must hold the agent, the tier and the playbook only", e)
			}
		}
	}
}

// playbook carries the effective fan-out cap of every harness, so each
// orchestrator reads it from data rather than prose.
func TestPlaybookListCarriesParallelCapForEveryHarness(t *testing.T) {
	d := fullDeps()
	d.Harnesses = func() []string {
		var ids []string
		for _, n := range core.AllHarnessCaps() {
			ids = append(ids, n.Harness)
		}
		return ids
	}
	d.Parallel = func(string) ParallelCap { return ParallelCap{Max: 6, WhenStrained: 3, Source: "default"} }
	env, err := call(t, d, "playbook", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var l playbookList
	if err := json.Unmarshal(env["data"], &l); err != nil {
		t.Fatal(err)
	}
	for _, n := range core.AllHarnessCaps() {
		if c, ok := l.MaxParallel[n.Harness]; !ok || c.Max != 6 || c.WhenStrained != 3 {
			t.Errorf("%s: cap = %+v (present %v)", n.Harness, c, ok)
		}
	}
}

func TestPlaybookListOmitsParallelWithoutSource(t *testing.T) {
	env, err := call(t, fullDeps(), "playbook", `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var l playbookList
	if err := json.Unmarshal(env["data"], &l); err != nil || l.MaxParallel != nil {
		t.Fatalf("err=%v cap=%v", err, l.MaxParallel)
	}
}
