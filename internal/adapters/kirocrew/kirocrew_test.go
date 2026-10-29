// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package kirocrew_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

var cat = catalog.Load()

const (
	smallID    = "claude-haiku-4-5"
	midID      = "claude-sonnet-5-5"
	frontierID = "claude-opus-5-5"
)

// writeSessionFile installs a hermetic kirocrew allowlist for one test.
func writeSessionFile(t *testing.T, ids []string) {
	t.Helper()
	data, err := json.Marshal(map[string][]string{"kirocrew": ids})
	if err != nil {
		t.Fatalf("marshal session: %v", err)
	}
	path := filepath.Join(t.TempDir(), "session-models.json")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write session file: %v", err)
	}
	t.Setenv("DOWNSHIFT_SESSION_MODELS", path)
}

func spawnEvent(task, model string) kirocrew.Event {
	input := `{"task": "` + task + `"`
	if model != "" {
		input += `, "model": "` + model + `"`
	}
	input += `}`
	return kirocrew.Event{
		ToolName:  "spawn_run",
		ToolInput: json.RawMessage(input),
	}
}

// fixedResolver overrides ModelFor and/or IsExplicitOnly on the real catalog
// to build held or explicit-only decisions on demand.
type fixedResolver struct {
	core.Resolver
	modelFor     *core.Model
	explicitOnly map[string]bool
}

func (s fixedResolver) ModelFor(harness string, tier core.Tier) core.Model {
	if s.modelFor != nil {
		return *s.modelFor
	}
	return s.Resolver.ModelFor(harness, tier)
}

func (s fixedResolver) IsExplicitOnly(harness, id string) bool {
	if s.explicitOnly != nil {
		if v, ok := s.explicitOnly[id]; ok {
			return v
		}
	}
	return s.Resolver.IsExplicitOnly(harness, id)
}

// The agent calls a spawn through its MCP server, so the hook sees the prefixed
// name. It must be routed exactly like the plain name.
func TestHandle_RoutesMCPPrefixedSpawn(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	for _, name := range []string{"@kirocrew-core/spawn_run", "spawn_run"} {
		ev := spawnEvent("rename the userId variable to userIdentifier", frontierID)
		ev.ToolName = name
		out, _, decision := kirocrew.Handle(ev, cat)
		if decision.Harness == "" {
			t.Fatalf("%s: expected a routing decision, got none", name)
		}
		if !out.Block {
			t.Errorf("%s: expected Block=true for the in-session held-free target", name)
		}
	}
}

// A prefixed tool that is not a spawn passes through with no decision.
func TestHandle_PassesThroughPrefixedNonSpawn(t *testing.T) {
	ev := spawnEvent("list the open tickets", frontierID)
	ev.ToolName = "@kirocrew-core/tool_search"
	out, _, decision := kirocrew.Handle(ev, cat)
	if out.Block || decision.Harness != "" {
		t.Errorf("a non-spawn must pass through: block=%v decision=%q", out.Block, decision.Harness)
	}
}

func TestHandle_BlocksConfidentMismatchInSession(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	ev := spawnEvent("rename the userId variable to userIdentifier", frontierID)

	out, note, decision := kirocrew.Handle(ev, cat)
	if decision.Harness == "" {
		t.Fatal("expected a routing decision, got none")
	}
	if len(decision.Corrections) != 0 {
		t.Fatalf("expected a held-free decision, got %v", decision.Corrections)
	}
	if !out.Block {
		t.Fatal("expected Block=true for an in-session held-free target")
	}
	if note == "" {
		t.Fatal("expected a block note, got none")
	}
	if !strings.Contains(out.Message, smallID) {
		t.Errorf("block message %q must name the in-session target %s", out.Message, smallID)
	}
}

func TestHandle_AllowsOutOfSessionTarget(t *testing.T) {
	// The recommended small model is not in this session: blocking would
	// tell the agent to respawn with an id it does not have.
	writeSessionFile(t, []string{frontierID})
	ev := spawnEvent("rename the userId variable to userIdentifier", frontierID)

	out, _, decision := kirocrew.Handle(ev, cat)
	if decision.Harness == "" {
		t.Fatal("expected a routing decision, got none")
	}
	if out.Block {
		t.Errorf("must not block when the target is outside the session (message=%q)", out.Message)
	}
	if out.Message != "" {
		t.Errorf("allow must carry no message, got %q", out.Message)
	}
}

func TestHandle_AllowsHeldDecision(t *testing.T) {
	// A held decision on the same spawn the block test names: Corrections
	// is non-empty, so the hook allows instead of blocking.
	decidertest.Use(t, decidertest.Held(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	res := fixedResolver{
		Resolver: cat,
		modelFor: &core.Model{ID: smallID, Harness: "cursor", Tier: core.TierSmall},
	}
	ev := spawnEvent("rename the userId variable to userIdentifier", frontierID)

	out, _, decision := kirocrew.Handle(ev, res)
	if len(decision.Corrections) == 0 {
		t.Fatal("expected a held decision, got none — test is vacuous")
	}
	if out.Block {
		t.Errorf("must not block a held decision (corrections=%v)", decision.Corrections)
	}
}

func TestHandle_SkipsExplicitOnlyTargetForNextSessionModel(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	res := fixedResolver{
		Resolver:     cat,
		explicitOnly: map[string]bool{smallID: true},
	}
	ev := spawnEvent("rename the userId variable to userIdentifier", frontierID)

	out, _, decision := kirocrew.Handle(ev, res)
	if len(decision.Corrections) != 0 {
		t.Fatalf("expected a held-free decision, got %v", decision.Corrections)
	}
	if !out.Block || !strings.Contains(out.Message, "model="+midID) {
		t.Errorf("must use the next non-explicit session model, got block=%t message=%q", out.Block, out.Message)
	}
}

func TestHandle_AllowsUnknownModel(t *testing.T) {
	writeSessionFile(t, []string{smallID, midID, frontierID})
	ev := spawnEvent("rename the userId variable to userIdentifier", "")

	out, note, _ := kirocrew.Handle(ev, cat)
	if out.Block {
		t.Error("must not block when the current model is unknown")
	}
	if note != "" {
		t.Errorf("expected no note for unknown model, got %q", note)
	}
}

func TestMain(m *testing.M) {
	os.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(os.TempDir(), "downshift-test-state-kirocrew"))
	os.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(os.TempDir(), "downshift-session-models-absent.json"))
	os.Setenv("DOWNSHIFT_DISCOVERY", "off")
	os.Exit(m.Run())
}

// A current model marked explicit_only is the user's deliberate choice: the
// hook must neither block it nor ask for a respawn on another model.
func TestHandle_AllowsExplicitOnlyCurrentModel(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	res := fixedResolver{
		Resolver:     cat,
		explicitOnly: map[string]bool{frontierID: true},
	}
	ev := spawnEvent("rename the userId variable to userIdentifier", frontierID)

	out, _, decision := kirocrew.Handle(ev, res)
	if decision.Intent != core.PreservedIntent {
		t.Fatalf("precondition: want preserved intent, got %v", decision.Intent)
	}
	if out.Block {
		t.Fatalf("must not block an explicit_only current model (message=%q)", out.Message)
	}
}

// Only KiroCrew spawn tools are routed. A tool that carries model and prompt
// fields (an image generator, for example) is not a subagent spawn and must
// pass through with no decision and no note, so nothing is logged or blocked.
func TestHandle_SpawnToolGate(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	writeSessionFile(t, []string{smallID, midID, frontierID})
	const (
		task   = "rename the userId variable to userIdentifier"
		prompt = "a red bicycle on a beach at sunset"
	)
	cases := []struct {
		name      string
		toolName  string
		input     string
		wantRoute bool
	}{
		{"spawn_run with model and task is a spawn", "spawn_run",
			`{"task": "` + task + `", "model": "` + frontierID + `"}`, true},
		{"spawn_sub_agents with model and task is a spawn", "spawn_sub_agents",
			`{"task": "` + task + `", "model": "` + frontierID + `"}`, true},
		{"non-spawn tool with model and prompt is not a spawn", "mcp_images_generate",
			`{"prompt": "` + prompt + `", "model": "` + frontierID + `"}`, false},
		{"missing tool name is not a spawn", "",
			`{"task": "` + task + `", "model": "` + frontierID + `"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := kirocrew.Event{ToolName: tc.toolName, ToolInput: json.RawMessage(tc.input)}
			out, note, decision := kirocrew.Handle(ev, cat)
			if tc.wantRoute {
				if decision.Harness == "" {
					t.Fatal("expected a routing decision for a spawn, got none")
				}
				if !out.Block {
					t.Errorf("expected the mismatched spawn to be blocked, message=%q", out.Message)
				}
				return
			}
			if decision.Harness != "" {
				t.Errorf("expected no routing decision, got harness %q", decision.Harness)
			}
			if out.Block || out.Message != "" || note != "" {
				t.Errorf("expected pass-through with no block, message or note; got block=%t message=%q note=%q",
					out.Block, out.Message, note)
			}
		})
	}
}
