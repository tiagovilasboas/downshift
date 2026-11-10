// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/core/decidertest"
)

// conformanceScenario is one harness-free behaviour. skip names the
// capability that excuses a harness; it never names a harness. decider, when
// set, pins the decision a contract scenario checks the wire against (a stub
// from decidertest). nil runs the scenario under the default v1 decider: only
// scenarios that check v1 policy (the project floor) leave it unset.
type conformanceScenario struct {
	name    string
	skip    func(caps core.HarnessCapabilities) string
	run     func(t *testing.T, h conformanceHarness)
	decider core.Decider
}

var conformanceScenarios = []conformanceScenario{
	{
		name:    "trivial task on the frontier moves to the small tier",
		decider: decidertest.Tier(core.TierSmall),
		skip: func(caps core.HarnessCapabilities) string {
			if !caps.CanRewriteModel && !caps.CanAdviseShellWrap {
				return "harness is observation-only (cannot rewrite or advise)"
			}
			return ""
		},
		run: func(t *testing.T, h conformanceHarness) {
			h.session(t, h.small, h.mid, h.frontier)
			res := h.run(t, spawnSpec{task: conformanceTrivialTask, model: h.frontier.ID})
			if !res.changed || res.model != h.small.WriteName() {
				t.Fatalf("got changed=%v model=%q rc=%d, want %q", res.changed, res.model, res.rc, h.small.WriteName())
			}
			if h.caps.CanRewriteModel && h.caps.RewritePreservesInput {
				if res.rc != 0 || res.fields == nil {
					t.Fatalf("a rewriting harness must allow (rc 0) with a rewritten input, got rc=%d fields=%v", res.rc, res.fields)
				}
			} else if res.fields != nil {
				t.Fatalf("a harness that does not echo the spawn's tool input must not emit a rewritten input, got %v", res.fields)
			}
			if h.caps.CanApplyEffort {
				if effort, _ := res.fields["reasoning_effort"].(string); effort == "" {
					t.Errorf("an effort-capable harness must write reasoning_effort on a downshift, got %v", res.fields)
				}
			}
			if ev := h.lastEvent(t); ev.Verdict != "DOWNSHIFT" || ev.ToModel != h.small.ID {
				t.Errorf("telemetry verdict=%q final_model=%q, want DOWNSHIFT to %s", ev.Verdict, ev.ToModel, h.small.ID)
			}
		},
	},
	{
		name:    "complex task on the small tier moves to the frontier",
		decider: decidertest.Tier(core.TierFrontier),
		skip: func(caps core.HarnessCapabilities) string {
			if !caps.CanRewriteModel && !caps.CanAdviseShellWrap {
				return "harness is observation-only (cannot rewrite or advise)"
			}
			return ""
		},
		run: func(t *testing.T, h conformanceHarness) {
			h.session(t, h.small, h.mid, h.frontier)
			res := h.run(t, spawnSpec{task: conformanceComplexTask, model: h.small.ID})
			if !res.changed || res.model != h.frontier.WriteName() {
				t.Fatalf("got changed=%v model=%q rc=%d, want %q", res.changed, res.model, res.rc, h.frontier.WriteName())
			}
			if ev := h.lastEvent(t); ev.Verdict != "UPSHIFT" {
				t.Errorf("telemetry verdict=%q, want UPSHIFT", ev.Verdict)
			}
		},
	},
	{
		name:    "right-sized task keeps its model",
		decider: decidertest.Tier(core.TierMid),
		run: func(t *testing.T, h conformanceHarness) {
			h.session(t, h.small, h.mid, h.frontier)
			res := h.run(t, spawnSpec{task: conformanceMediumTask, model: h.mid.ID})
			if res.changed || res.rc != 0 {
				t.Fatalf("got changed=%v model=%q rc=%d, want the spawn allowed unchanged", res.changed, res.model, res.rc)
			}
			if h.caps.CanRewriteModel && !h.caps.CanApplyEffort && res.fields != nil {
				t.Fatalf("no model change must mean no rewritten input, got %v", res.fields)
			}
			if ev := h.lastEvent(t); ev.Verdict != "OK" {
				t.Errorf("telemetry verdict=%q, want OK", ev.Verdict)
			}
		},
	},
	{
		name:    "sibling tool-input fields survive the rewrite",
		decider: decidertest.Tier(core.TierSmall),
		skip: func(caps core.HarnessCapabilities) string {
			if !caps.CanRewriteModel {
				return "harness has no rewrite channel (CanRewriteModel=false)"
			}
			if !caps.RewritePreservesInput {
				return "harness wire does not echo the spawn's tool input"
			}
			return ""
		},
		run: func(t *testing.T, h conformanceHarness) {
			h.session(t, h.small, h.mid, h.frontier)
			siblings := map[string]any{
				"description":       "a short rename task",
				"timeout":           30,
				"run_in_background": false,
				"tags":              []any{"a", "b"},
				"meta":              map[string]any{"k": "v"},
			}
			res := h.run(t, spawnSpec{task: conformanceTrivialTask, model: h.frontier.ID, siblings: siblings})
			if res.fields == nil || res.model != h.small.WriteName() {
				t.Fatalf("expected a rewrite to %q, got %+v", h.small.WriteName(), res)
			}
			// Round-trip the expected values through JSON so numbers compare
			// as the decoder sees them.
			var want map[string]any
			if err := json.Unmarshal(mustJSON(siblings), &want); err != nil {
				t.Fatal(err)
			}
			for k, v := range want {
				if got, ok := res.fields[k]; !ok {
					t.Errorf("sibling %q was dropped from the rewritten input: %v", k, res.fields)
				} else if !reflect.DeepEqual(got, v) {
					t.Errorf("sibling %q = %v, want %v", k, got, v)
				}
			}
			if res.fields["description"] == nil {
				t.Errorf("task text must survive the rewrite: %v", res.fields)
			}
		},
	},
	{
		name:    "missing session list never rewrites (unless SessionFromCatalog)",
		decider: decidertest.Tier(core.TierSmall),
		skip: func(caps core.HarnessCapabilities) string {
			// Observation-only harnesses never rewrite, so this scenario only
			// makes sense for harnesses that can rewrite or advise.
			if !caps.CanRewriteModel && !caps.CanAdviseShellWrap && !caps.SessionFromCatalog {
				return "harness is observation-only (cannot rewrite or advise)"
			}
			return ""
		},
		run: func(t *testing.T, h conformanceHarness) {
			// No allowlist anywhere: the session file is absent (see
			// conformanceEnv). Nothing but a listing or the payload supplies
			// candidates, so the spawn is allowed unchanged. A harness whose
			// payload never carries a list (SessionFromCatalog) builds one
			// from the catalog candidates below the current model instead.
			res := h.run(t, spawnSpec{task: conformanceTrivialTask, model: h.frontier.ID})
			if h.caps.SessionFromCatalog {
				if !res.changed || res.rc != 0 || res.model != h.small.WriteName() {
					t.Fatalf("catalog candidates must downshift to %q, got %+v", h.small.WriteName(), res)
				}
				return
			}
			if res.changed || res.rc != 0 || res.fields != nil {
				t.Fatalf("unknown session must allow unchanged, got %+v", res)
			}
		},
	},
	{
		name:    "project floor raises the tier",
		decider: routingDecider{now: time.Now},
		skip: func(caps core.HarnessCapabilities) string {
			if !caps.CanRewriteModel && !caps.CanAdviseShellWrap {
				return "harness is observation-only (cannot rewrite or advise)"
			}
			return ""
		},
		run: func(t *testing.T, h conformanceHarness) {
			h.session(t, h.small, h.mid, h.frontier)
			project := t.TempDir()
			writeProjectFloor(t, project, core.TierMid.String())
			res := h.run(t, spawnSpec{task: conformanceTrivialTask, model: h.frontier.ID, cwd: project})
			if !res.changed || res.model != h.mid.WriteName() {
				t.Fatalf("with a mid floor got changed=%v model=%q rc=%d, want %q", res.changed, res.model, res.rc, h.mid.WriteName())
			}
			if ev := h.lastEvent(t); !ev.PolicyFloor || ev.ToModel != h.mid.ID {
				t.Errorf("telemetry policy_floor=%v final_model=%q, want the floor recorded on %s", ev.PolicyFloor, ev.ToModel, h.mid.ID)
			}
			// Same spawn without the floor: the floor came from the project
			// context, not from the payload or the session.
			res = h.run(t, spawnSpec{task: conformanceTrivialTask, model: h.frontier.ID})
			if !res.changed || res.model != h.small.WriteName() {
				t.Fatalf("without a floor got changed=%v model=%q, want %q", res.changed, res.model, h.small.WriteName())
			}
			if ev := h.lastEvent(t); ev.PolicyFloor {
				t.Errorf("policy_floor must be recorded only on the raised decision")
			}
		},
	},
	{
		name:    "session without the small tier uses the next session model",
		decider: decidertest.Tier(core.TierSmall),
		skip: func(caps core.HarnessCapabilities) string {
			if !caps.CanRewriteModel && !caps.CanAdviseShellWrap {
				return "harness is observation-only (cannot rewrite or advise)"
			}
			return ""
		},
		run: func(t *testing.T, h conformanceHarness) {
			h.session(t, h.mid, h.frontier)
			res := h.run(t, spawnSpec{task: conformanceTrivialTask, model: h.frontier.ID})
			if !res.changed || res.model != h.mid.WriteName() {
				t.Fatalf("got changed=%v model=%q rc=%d, want the session's next model %q", res.changed, res.model, res.rc, h.mid.WriteName())
			}
		},
	},
	{
		name:    "session's own small model wins over the catalog default",
		decider: decidertest.Tier(core.TierSmall),
		skip: func(caps core.HarnessCapabilities) string {
			if !caps.CanRewriteModel && !caps.CanAdviseShellWrap {
				return "harness is observation-only (cannot rewrite or advise)"
			}
			return ""
		},
		run: func(t *testing.T, h conformanceHarness) {
			alt := h.alternateSmall()
			if alt.ID == "" {
				t.Skipf("catalog has one writable small model for %s", h.id)
			}
			h.session(t, alt, h.mid, h.frontier)
			res := h.run(t, spawnSpec{task: conformanceTrivialTask, model: h.frontier.ID})
			if !res.changed || res.model != alt.WriteName() {
				t.Fatalf("got changed=%v model=%q rc=%d, want the session's %q (not the catalog default %q)", res.changed, res.model, res.rc, alt.WriteName(), h.small.WriteName())
			}
		},
	},
}
