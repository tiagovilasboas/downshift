// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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

func TestHandle_BlocksConfidentMismatchInSession(t *testing.T) {
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
	// R4 foreign-model hold: same id the block test names, but recommended
	// from another harness, so Corrections is non-empty.
	writeSessionFile(t, []string{smallID, midID, frontierID})
	res := fixedResolver{
		Resolver: cat,
		modelFor: &core.Model{ID: smallID, Harness: "cursor", Tier: core.TierSmall},
	}
	ev := spawnEvent("rename the userId variable to userIdentifier", frontierID)

	out, _, decision := kirocrew.Handle(ev, res)
	if len(decision.Corrections) == 0 {
		t.Fatal("expected a held decision (R4), got none — test is vacuous")
	}
	if out.Block {
		t.Errorf("must not block a held decision (corrections=%v)", decision.Corrections)
	}
}

func TestHandle_SkipsExplicitOnlyTargetForNextSessionModel(t *testing.T) {
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
	os.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(os.TempDir(), "downshift-session-models-absent.json"))
	os.Setenv("DOWNSHIFT_DISCOVERY", "off")
	os.Exit(m.Run())
}

// A current model marked explicit_only is the user's deliberate choice: the
// hook must neither block it nor ask for a respawn on another model.
func TestHandle_AllowsExplicitOnlyCurrentModel(t *testing.T) {
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
