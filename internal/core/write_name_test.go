// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package core_test

import (
	"testing"

	"github.com/tiagovilasboas/harness-downshift/internal/core"
)

func TestModel_WriteName(t *testing.T) {
	if got := (core.Model{ID: "m-1", Native: "m"}).WriteName(); got != "m" {
		t.Errorf("with native name: %q, want m", got)
	}
	if got := (core.Model{ID: "m-1"}).WriteName(); got != "m-1" {
		t.Errorf("without native name: %q, want the id", got)
	}
}

// noNative strips the native name from every model, as a catalog entry that
// omits native_name would.
type noNative struct{ core.Resolver }

func (r noNative) ModelFor(h string, t core.Tier) core.Model {
	m := r.Resolver.ModelFor(h, t)
	m.Native = ""
	return m
}

func (r noNative) LookupByID(h, id string) (core.Model, bool) {
	m, ok := r.Resolver.LookupByID(h, id)
	m.Native = ""
	return m, ok
}

func planFor(t *testing.T, caps core.HarnessCapabilities, res core.Resolver) core.RewritePlan {
	t.Helper()
	small := cat.ModelFor("claude-code", core.TierSmall).ID
	frontier := cat.ModelFor("claude-code", core.TierFrontier).ID
	d := core.Route("rename the userId variable", "claude-code", frontier, res)
	d.RequestedID = frontier
	session := core.KnownSession([]string{small, frontier})
	return d.PlanForSession(caps, res, session)
}

func TestPlan_StrictHarnessWritesNativeName(t *testing.T) {
	plan := planFor(t, core.ClaudeCodeCaps, cat)
	if !plan.RewriteModel {
		t.Fatal("expected a rewrite")
	}
	want := cat.ModelFor("claude-code", core.TierSmall).Native
	if want == "" || plan.WriteName != want {
		t.Fatalf("WriteName = %q, want the catalog native name %q", plan.WriteName, want)
	}
}

// A strict harness rejects any other string and blocks the spawn, so a target
// without a native name must not be written.
func TestPlan_StrictHarnessWithoutNativeNameDoesNotRewrite(t *testing.T) {
	plan := planFor(t, core.ClaudeCodeCaps, noNative{cat})
	if plan.RewriteModel || plan.WriteName != "" {
		t.Fatalf("strict harness must not rewrite without a native name: %+v", plan)
	}
}

// A non-strict harness falls back to the id.
func TestPlan_NonStrictHarnessFallsBackToID(t *testing.T) {
	caps := core.ClaudeCodeCaps
	caps.StrictModelName = false
	plan := planFor(t, caps, noNative{cat})
	if !plan.RewriteModel || plan.WriteName != plan.Model.ID {
		t.Fatalf("want rewrite with the id, got %+v", plan)
	}
}
