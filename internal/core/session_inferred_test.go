// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

func TestInferSessionFromCurrent_LadderEndsAtCurrent(t *testing.T) {
	s := core.InferSessionFromCurrent("claude-code", "claude-opus-5-5", cat)
	if !s.Known || !s.Inferred || len(s.IDs) == 0 {
		t.Fatalf("want known inferred session, got %+v", s)
	}
	if last := s.IDs[len(s.IDs)-1]; last != "claude-opus-5-5" {
		t.Fatalf("ladder must end at the current model, got %v", s.IDs)
	}
	low := core.InferSessionFromCurrent("claude-code", "claude-haiku-5-5", cat)
	if len(low.IDs) != 1 || low.IDs[0] != "claude-haiku-5-5" {
		t.Fatalf("smallest model has nothing below it, got %v", low.IDs)
	}
}

func TestInferSessionFromCurrent_UnknownStaysUnknown(t *testing.T) {
	for _, cur := range []string{"", "no-such-model"} {
		if s := core.InferSessionFromCurrent("claude-code", cur, cat); s.Known {
			t.Fatalf("current %q must not infer a session: %+v", cur, s)
		}
	}
	t.Setenv(core.EnvInferSession, "off")
	if s := core.InferSessionFromCurrent("claude-code", "claude-opus-5-5", cat); s.Known {
		t.Fatalf("layer disabled must stay unknown")
	}
}

// With no session list at all, a trivial task on the current opus session is
// downshifted inside the inferred ladder and never goes above the current model.
func TestPlanSession_InfersWhenSessionUnknown(t *testing.T) {
	d := core.Route("rename the userId variable", "claude-code", "claude-opus-5-5", cat)
	plan := d.PlanSession(core.ClaudeCodeCaps, cat, core.UnknownSession(), "claude-opus-5-5")
	if d.Session.Unknown {
		t.Fatalf("decision must record the inferred session as known")
	}
	if !plan.RewriteModel || plan.Model.ID == "claude-opus-5-5" {
		t.Fatalf("want a downshift from opus, got %+v", plan)
	}
	for _, id := range core.InferSessionFromCurrent("claude-code", "claude-opus-5-5", cat).IDs {
		if id == plan.Model.ID {
			return
		}
	}
	t.Fatalf("target %q outside the inferred ladder", plan.Model.ID)
}

func TestPlanSession_NoCurrentModelStillHolds(t *testing.T) {
	d := core.Route("rename the userId variable", "claude-code", "", cat)
	plan := d.PlanSession(core.ClaudeCodeCaps, cat, core.UnknownSession(), "")
	if plan.RewriteModel || !d.Session.Unknown {
		t.Fatalf("no current model must not rewrite: %+v", plan)
	}
}

// Harnesses whose model facts do not prove access keep an unknown session
// unknown: the capability is data, not a branch.
func TestPlanSession_InferenceIsACapability(t *testing.T) {
	d := core.Route("rename the userId variable", "cursor", "claude-4.5-opus-high-thinking", cat)
	plan := d.PlanSession(core.CursorCaps, cat, core.UnknownSession(), "claude-4.5-opus-high-thinking")
	if plan.RewriteModel || !d.Session.Unknown {
		t.Fatalf("a harness without InferSession must not infer: %+v", plan)
	}
}
