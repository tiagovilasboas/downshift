// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// Harness conformance suite. One table of harness-free routing scenarios runs
// against every hook in harnessRegistry through the shared runner, so a
// behaviour that holds for one harness is proved for all of them in one place
// (AGENTS.md layering rule 4: test at the shared layer). The only per-harness
// code is data: how to build that harness's spawn payload and how to read its
// wire response back into a hookResult. A scenario is skipped only on a
// declared capability or on catalog data, never on a harness name.

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const (
	conformanceTrivialTask = "rename the userId variable to userIdentifier"
	conformanceMediumTask  = "implement the CSV export feature"
	conformanceComplexTask = "rearchitect the payment flow across services"
)

// spawnSpec is one subagent spawn in harness-free terms.
type spawnSpec struct {
	task     string
	model    string         // requested model id; "" means the spawn names none
	cwd      string         // working directory the payload reports
	siblings map[string]any // extra tool-input fields beside the task and model
}

// hookResult is the harness-free reading of one hook response.
type hookResult struct {
	rc      int
	changed bool           // the hook asked for a model other than the requested one (a rewrite, or a block naming one)
	model   string         // the model the hook left on the wire, or named in a block
	fields  map[string]any // the rewritten tool input; nil when no rewrite was emitted
}

// conformanceDriver is the per-harness data: payload shape in, wire shape out.
type conformanceDriver struct {
	build  func(spawnSpec) []byte
	decode func(t *testing.T, rc int, stdout, stderr, requested string) hookResult
	// chdir means the payload carries no working directory, so the runner
	// reads the process cwd (the project seam) instead; the suite chdirs to
	// the scenario's cwd before running the hook.
	chdir bool
}

// conformanceChildHex is the synthetic spawn correlation for the opencode
// driver: lowercase hex, within the wire's 16..64 character contract.
const conformanceChildHex = "a1b2c3d4e5f60718293a4b5c6d7e8f90"

// conformanceDrivers maps a registry id to its payload builder and response
// decoder. Every registry entry with a Hook must have one; the suite fails
// otherwise, so a new harness cannot be left out of the shared proof.
var conformanceDrivers = map[string]conformanceDriver{
	"claude-code": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "Task",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("prompt", s),
			})
		},
		decode: decodeHookSpecificOutput,
	},
	"codex": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"hook_event_name": "PreToolUse",
				"tool_name":       "spawn_agent",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("message", s),
			})
		},
		decode: decodeHookSpecificOutput,
	},
	"cursor": {
		build: func(s spawnSpec) []byte {
			payload := map[string]any{
				"hook_event_name": "preToolUse",
				"tool_name":       "Task",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("task", s),
			}
			if s.model != "" {
				payload["model_id"] = s.model
			}
			return mustJSON(payload)
		},
		decode: func(t *testing.T, rc int, stdout, _, requested string) hookResult {
			t.Helper()
			var out struct {
				UpdatedInput map[string]any `json:"updated_input"`
			}
			decodeHookStdout(t, stdout, &out)
			return resultFromFields(rc, out.UpdatedInput, "model", requested)
		},
	},
	"antigravity": {
		build: func(s spawnSpec) []byte {
			sub := map[string]any{"Prompt": s.task}
			if s.model != "" {
				sub["Model"] = s.model
			}
			for k, v := range s.siblings {
				sub[k] = v
			}
			return mustJSON(map[string]any{
				"cwd": s.cwd,
				"toolCall": map[string]any{
					"name": "invoke_subagent",
					"args": map[string]any{"Subagents": []any{sub}},
				},
			})
		},
		decode: func(t *testing.T, rc int, stdout, _, requested string) hookResult {
			t.Helper()
			var out struct {
				Overwrite *struct {
					Subagents []map[string]any `json:"Subagents"`
				} `json:"overwrite"`
			}
			decodeHookStdout(t, stdout, &out)
			if out.Overwrite == nil || len(out.Overwrite.Subagents) == 0 {
				return hookResult{rc: rc}
			}
			return resultFromFields(rc, out.Overwrite.Subagents[len(out.Overwrite.Subagents)-1], "Model", requested)
		},
	},
	"kirocrew": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"hook_event_name": "preToolUse",
				"tool_name":       "spawn_run",
				"cwd":             s.cwd,
				"tool_input":      conformanceToolInput("task", s),
			})
		},
		// No rewrite channel: exit 2 blocks and stderr names the model to
		// respawn with ("model=<id>"); exit 0 allows unchanged.
		decode: func(_ *testing.T, rc int, _, stderr, _ string) hookResult {
			res := hookResult{rc: rc}
			if rc != 2 {
				return res
			}
			res.changed = true
			if _, after, ok := strings.Cut(stderr, "model="); ok {
				res.model = strings.FieldsFunc(after, func(r rune) bool { return r == ' ' || r == '\n' || r == '(' })[0]
			}
			return res
		},
	},
	"opencode": {
		build: func(s spawnSpec) []byte {
			return mustJSON(map[string]any{
				"v":           1,
				"kind":        "route",
				"harness":     map[string]any{"id": "opencode", "version": "2.0.32"},
				"event":       "subagent_spawn_intent",
				"correlation": map[string]any{"child": conformanceChildHex, "parent": "f0e1d2c3b4a5968778695a4b3c2d1e0f"},
				"agent":       "general",
				"model":       map[string]any{"providerID": "opencode", "id": s.model, "variant": "default"},
				"taskText":    s.task,
				"tsUtc":       "2026-10-18T12:00:00Z",
			})
		},
		// The route answer names the model the plugin should spawn on; it
		// never echoes the spawn input, so fields stay nil. Action "route"
		// is the only change signal on this wire.
		decode: func(t *testing.T, rc int, stdout, _, requested string) hookResult {
			t.Helper()
			var out struct {
				V      int    `json:"v"`
				Action string `json:"action"`
				Model  *struct {
					ProviderID string `json:"providerID"`
					ID         string `json:"id"`
				} `json:"model"`
			}
			decodeHookStdout(t, stdout, &out)
			res := hookResult{rc: rc}
			if out.Action == "route" && out.Model != nil && out.Model.ID != "" {
				res.changed = out.Model.ID != requested
				res.model = out.Model.ID
			}
			return res
		},
		chdir: true,
	},
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// conformanceToolInput builds a flat tool input: the task under taskKey, the
// model under "model" when present, and every sibling field beside them.
func conformanceToolInput(taskKey string, s spawnSpec) map[string]any {
	m := map[string]any{taskKey: s.task}
	if s.model != "" {
		m["model"] = s.model
	}
	for k, v := range s.siblings {
		m[k] = v
	}
	return m
}

func decodeHookStdout(t *testing.T, stdout string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), into); err != nil {
		t.Fatalf("hook stdout is not one JSON object: %v\n%s", err, stdout)
	}
}

// resultFromFields reads a rewritten tool input. An input whose model equals
// the requested one is not a model change (an effort-only rewrite, for example).
func resultFromFields(rc int, fields map[string]any, modelKey, requested string) hookResult {
	if fields == nil {
		return hookResult{rc: rc}
	}
	model, _ := fields[modelKey].(string)
	return hookResult{rc: rc, changed: model != "" && model != requested, model: model, fields: fields}
}

// decodeHookSpecificOutput reads the Claude Code / Codex PreToolUse envelope.
func decodeHookSpecificOutput(t *testing.T, rc int, stdout, _, requested string) hookResult {
	t.Helper()
	var out struct {
		HookSpecificOutput *struct {
			PermissionDecision string         `json:"permissionDecision"`
			UpdatedInput       map[string]any `json:"updatedInput"`
		} `json:"hookSpecificOutput"`
	}
	decodeHookStdout(t, stdout, &out)
	if out.HookSpecificOutput == nil {
		return hookResult{rc: rc}
	}
	// A decision without a rewrite would override the user's ask rules.
	if out.HookSpecificOutput.PermissionDecision != "" && out.HookSpecificOutput.UpdatedInput == nil {
		t.Errorf("unchanged spawn must emit no permissionDecision: %s", stdout)
	}
	return resultFromFields(rc, out.HookSpecificOutput.UpdatedInput, "model", requested)
}

// conformanceHarness is one harness under test: registry facts plus the
// catalog models of each tier.
type conformanceHarness struct {
	id                   string
	caps                 core.HarnessCapabilities
	driver               conformanceDriver
	small, mid, frontier core.Model
	eventLog             string
}

// run drives one spawn through the harness's registered hook (the shared
// runner) and decodes the response.
func (h conformanceHarness) run(t *testing.T, s spawnSpec) hookResult {
	t.Helper()
	if s.cwd == "" {
		s.cwd = t.TempDir()
	}
	if h.driver.chdir {
		t.Chdir(s.cwd)
	}
	hook := harnessRegistry[h.id].Hook
	var rc int
	var stderr string
	stdout := captureStdout(func() {
		stderr = captureStderr(func() { rc = hook(bytes.NewReader(h.driver.build(s)), cmdCat) })
	})
	return h.driver.decode(t, rc, stdout, stderr, s.model)
}

// session installs the session allowlist the hook will resolve for this harness.
func (h conformanceHarness) session(t *testing.T, models ...core.Model) {
	t.Helper()
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	setSessionAllowlist(t, h.id, ids)
}

// alternateSmall returns a second automatic small-tier catalog model of this
// harness, distinct from the default, that the harness can write. Zero when
// the catalog has none. Candidates are walked sorted by id: the catalog slice
// is shared in-process and another command may have reordered it, so the
// scenario's alternate must not depend on slice order.
func (h conformanceHarness) alternateSmall() core.Model {
	var candidates []core.Model
	for _, e := range cmdCat.Entries() {
		if e.Harness != h.id || e.Tier != core.TierSmall.String() || e.ID == h.small.ID || e.Routing == "explicit_only" {
			continue
		}
		if h.caps.StrictModelName && e.NativeName == "" {
			continue
		}
		if m, ok := cmdCat.LookupByID(h.id, e.ID); ok && m.ID == e.ID {
			candidates = append(candidates, m)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	if len(candidates) == 0 {
		return core.Model{}
	}
	return candidates[0]
}

// lastEvent returns the newest telemetry event the hook recorded.
func (h conformanceHarness) lastEvent(t *testing.T) telemetry.Event {
	t.Helper()
	events, err := telemetry.ReadEventsFrom(h.eventLog)
	if err != nil || len(events) == 0 {
		t.Fatalf("no telemetry event recorded (err=%v)", err)
	}
	return events[len(events)-1]
}

func writeProjectFloor(t *testing.T, dir, tier string) {
	t.Helper()
	p := filepath.Join(dir, ".downshift", "policy.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"version":1,"min_tier":"`+tier+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

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
		name: "project floor raises the tier",
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

// conformanceEnv isolates one scenario: fresh HOME and state dir, a private
// event log, and no session file until the scenario installs one.
func conformanceEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent-session-models.json"))
	eventLog := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", eventLog)
	return eventLog
}

// TestHarnessConformance runs every scenario against every hook in the
// registry. Harness ids come from the registry and core.AllHarnessCaps, never
// from a literal in a loop.
func TestHarnessConformance(t *testing.T) {
	runConformance(t)
}

func runConformance(t *testing.T) {
	t.Helper()
	// Every declared harness must be routable through the registry, and every
	// registered hook must have conformance data.
	for _, n := range core.AllHarnessCaps() {
		if e, ok := harnessRegistry[n.Harness]; !ok || e.Hook == nil {
			t.Fatalf("harness %q is declared in core.AllHarnessCaps but has no registry hook", n.Harness)
		}
	}
	for _, id := range harnessIDs() {
		entry := harnessRegistry[id]
		if entry.Hook == nil {
			continue // try-only harness: nothing to conform
		}
		driver, ok := conformanceDrivers[id]
		if !ok {
			t.Fatalf("registry harness %q has no conformance driver; add its payload builder and decoder", id)
		}
		t.Run(id, func(t *testing.T) {
			h := conformanceHarness{
				id:       id,
				caps:     entry.Caps,
				driver:   driver,
				small:    cmdCat.ModelFor(id, core.TierSmall),
				mid:      cmdCat.ModelFor(id, core.TierMid),
				frontier: cmdCat.ModelFor(id, core.TierFrontier),
			}
			if h.small.ID == "" || h.mid.ID == "" || h.frontier.ID == "" {
				t.Fatalf("catalog lacks a tier model for %s (small=%q mid=%q frontier=%q)", id, h.small.ID, h.mid.ID, h.frontier.ID)
			}
			for _, sc := range conformanceScenarios {
				t.Run(sc.name, func(t *testing.T) {
					if sc.decider != nil {
						decidertest.Use(t, sc.decider)
					}
					if sc.skip != nil {
						if reason := sc.skip(h.caps); reason != "" {
							t.Skip(reason)
						}
					}
					h.eventLog = conformanceEnv(t)
					sc.run(t, h)
				})
			}
		})
	}
}
