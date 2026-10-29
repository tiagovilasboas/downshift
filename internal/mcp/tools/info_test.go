// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/playbook"
)

// providers stores the model ids the orchestrator passes as a hint, so it is
// the one information tool that must not claim to be read-only.
func TestOnlyProvidersIsNotReadOnly(t *testing.T) {
	for _, tl := range Info(fullDeps()) {
		if tl.Title == "" || !tl.Idempotent {
			t.Errorf("%s: want a title and idempotent", tl.Name)
		}
		if want := tl.Name != "providers"; tl.ReadOnly != want {
			t.Errorf("%s: ReadOnly = %v, want %v", tl.Name, tl.ReadOnly, want)
		}
		// providers is the entry point the instructions name; no other tool is loaded up front.
		if want := tl.Name == "providers"; tl.AlwaysLoad != want {
			t.Errorf("%s: AlwaysLoad = %v, want %v", tl.Name, tl.AlwaysLoad, want)
		}
	}
}

func TestInfoToolsAreValidAndNamed(t *testing.T) {
	ts := Info(fullDeps())
	if err := Validate(ts); err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^[a-z0-9_]{1,40}$`)
	want := map[string]bool{"providers": true, "playbook": true, "status": true, "apply": true}
	for _, tl := range ts {
		if !re.MatchString(tl.Name) || !want[tl.Name] {
			t.Errorf("unexpected tool name %q", tl.Name)
		}
		if len(tl.Description) < 40 {
			t.Errorf("%s: description too short", tl.Name)
		}
		var s struct {
			Type string `json:"type"`
			AP   *bool  `json:"additionalProperties"`
		}
		if err := json.Unmarshal(tl.Schema(), &s); err != nil || s.Type != "object" || s.AP == nil || *s.AP {
			t.Errorf("%s: schema must be a closed object", tl.Name)
		}
	}
	if len(ts) != len(want) {
		t.Fatalf("got %d tools", len(ts))
	}
}

func TestEnvelopeAlwaysParses(t *testing.T) {
	for _, tc := range []struct{ name, args string }{
		{"providers", `{"harness":"codex"}`}, {"playbook", ``}, {"playbook", `{"name":"backend"}`},
		{"status", `null`}, {"apply", `{"harness":"codex","model":"m1"}`},
	} {
		if _, err := call(t, fullDeps(), tc.name, tc.args); err != nil {
			t.Errorf("%s %s: %v", tc.name, tc.args, err)
		}
		if _, err := call(t, Deps{}, tc.name, tc.args); err != nil {
			continue // a nil port fails the call, never the envelope
		}
	}
}

func TestArgumentErrors(t *testing.T) {
	d := fullDeps()
	mustFail(t, d, "providers", `{}`, `"harness" is required`)
	mustFail(t, d, "providers", `{"harness":"nope"}`, "unknown harness", "claude-code, codex")
	mustFail(t, d, "providers", `{"harness":3}`, "must be a string")
	mustFail(t, d, "providers", `[1]`, "JSON object", "harness")
	mustFail(t, d, "providers", `{"harness":"codex","x":1}`, "unknown field", "harness")
	mustFail(t, d, "status", `{"harness":"codex"}`, "allowed fields: none")
	mustFail(t, d, "playbook", `{"name":"zzz"}`, "unknown playbook", "General, Backend")
	mustFail(t, d, "apply", `{"harness":"codex","prompt":"do the secret thing"}`, "never takes task text", "harness, model")
	mustFail(t, d, "apply", `{"harness":"codex","unknown":"x"}`, "unknown field")
	mustFail(t, d, "apply", `{"model":"m1"}`, `"harness" is required`)
}

func TestNullAndEmptyArgsAreEmptyObject(t *testing.T) {
	for _, a := range []string{``, `null`, `  `, `{}`} {
		if _, err := call(t, fullDeps(), "status", a); err != nil {
			t.Errorf("status %q: %v", a, err)
		}
	}
}

func TestPortFailuresAreErrorsWithoutLeaks(t *testing.T) {
	d := fullDeps()
	d.Models = fakeModels{err: errPort}
	mustFail(t, d, "providers", `{"harness":"codex"}`, "could not read")
	d.Playbooks = fakePlaybooks{err: errPort}
	mustFail(t, d, "playbook", `{}`, "cannot be read")
	d.Applier = fakeApplier{err: errPort}
	mustFail(t, d, "apply", `{"harness":"codex"}`, "could not work out")
	mustFail(t, Deps{Harnesses: d.Harnesses}, "providers", `{"harness":"codex"}`, "not available")
	mustFail(t, Deps{Harnesses: d.Harnesses}, "apply", `{"harness":"codex"}`, "not available")
	mustFail(t, Deps{}, "playbook", `{}`, "shipped playbooks cannot be read")
}

func TestStrainAdvice(t *testing.T) {
	var got []string
	d := fullDeps()
	d.Strain = fakeStrain{entries: []StrainEntry{{Provider: "p", Headroom: "critical"}}, got: &got}
	env, err := call(t, d, "providers", `{"harness":"codex"}`)
	if err != nil || len(got) != 1 || got[0] != "codex" {
		t.Fatalf("providers strain ids = %v, err %v", got, err)
	}
	if !strings.Contains(string(env["strain"]), `"critical"`) {
		t.Fatalf("strain = %s", env["strain"])
	}
	if _, err := call(t, d, "status", `{}`); err != nil || len(got) != 2 {
		t.Fatalf("status strain ids = %v, err %v", got, err)
	}
	for name, st := range map[string]Strain{"nil": nil, "failing": fakeStrain{err: errPort}} {
		d.Strain = st
		env, err := call(t, d, "providers", `{"harness":"codex"}`)
		if err != nil {
			t.Fatalf("%s strain failed the call: %v", name, err)
		}
		if string(env["strain"]) != "[]" || string(env["strain_unavailable"]) != "true" {
			t.Errorf("%s strain: %s %s", name, env["strain"], env["strain_unavailable"])
		}
	}
}

func TestProvidersData(t *testing.T) {
	env, _ := call(t, fullDeps(), "providers", `{"harness":"claude-code"}`)
	var l ModelList
	if err := json.Unmarshal(env["data"], &l); err != nil || len(l.Models) != 2 || !l.Fallback || !l.Models[1].Untiered {
		t.Fatalf("data = %s", env["data"])
	}
}

func TestPlaybookData(t *testing.T) {
	env, _ := call(t, fullDeps(), "playbook", `{}`)
	var l playbookList
	_ = json.Unmarshal(env["data"], &l)
	if len(l.Playbooks) != 2 || l.Playbooks[1].Name != "Backend" || l.Playbooks[1].Chars != len("tests first\nsecond line") {
		t.Fatalf("list = %s", env["data"])
	}
	env, _ = call(t, fullDeps(), "playbook", `{"name":"BACKÉND"}`)
	var x playbookText
	_ = json.Unmarshal(env["data"], &x)
	if x.Name != "Backend" || x.General != "General" || !strings.HasPrefix(x.Text, "tests first") || x.Source != "edited" {
		t.Fatalf("text = %s", env["data"])
	}
}

func TestStatusData(t *testing.T) {
	env, _ := call(t, fullDeps(), "status", `{}`)
	var s statusData
	_ = json.Unmarshal(env["data"], &s)
	if s.Server != "downshift" || s.Version != "9.9.9" || s.Playbooks != 2 {
		t.Fatalf("status = %s", env["data"])
	}
	d := fullDeps()
	d.Playbooks = fakePlaybooks{err: errPort}
	env, _ = call(t, d, "status", `{}`)
	if !strings.Contains(string(env["data"]), "playbooks_unavailable") {
		t.Fatalf("status = %s", env["data"])
	}
}

func TestApplyPassesFieldsAndReturnsResult(t *testing.T) {
	var got ApplyRequest
	d := fullDeps()
	d.Applier = fakeApplier{out: ApplyOutcome{Harness: "codex", SelectedModel: "m2", Verdict: "downshift", Applied: true}, got: &got}
	env, err := call(t, d, "apply", `{"harness":"codex","model":"m1","effort":"high","tier":"small","agent_type":"x","dir":"d"}`)
	if err != nil {
		t.Fatal(err)
	}
	if got != (ApplyRequest{Harness: "codex", Model: "m1", Effort: "high", Tier: "small", AgentType: "x", Dir: "d"}) {
		t.Fatalf("request = %+v", got)
	}
	if !strings.Contains(string(env["data"]), `"applied":true`) {
		t.Fatalf("data = %s", env["data"])
	}
}

func TestTruncationAtItemBoundary(t *testing.T) {
	d := fullDeps()
	big := make([]Model, 2000)
	for i := range big {
		big[i] = Model{ID: strings.Repeat("x", 60), Tier: "mid", Source: "catalog"}
	}
	d.Models = fakeModels{list: ModelList{Harness: "codex", Models: big}}
	res, err := toolByName(t, d, "providers").Handler(t.Context(), json.RawMessage(`{"harness":"codex"}`))
	if err != nil || len(res.Text) > maxResultBytes {
		t.Fatalf("len %d, err %v", len(res.Text), err)
	}
	var env struct {
		Data      ModelList `json:"data"`
		Truncated bool      `json:"truncated"`
	}
	if err := json.Unmarshal([]byte(res.Text), &env); err != nil || !env.Truncated || len(env.Data.Models) == 0 {
		t.Fatalf("truncated result: %v %+v", err, env.Truncated)
	}

	long := strings.Repeat("line of text\n", 20000)
	d.Playbooks = fakePlaybooks{sets: []playbook.Set{{ID: "general", Name: "General", Text: long}}}
	res, err = toolByName(t, d, "playbook").Handler(t.Context(), json.RawMessage(`{"name":"general"}`))
	if err != nil || len(res.Text) > maxResultBytes || !strings.Contains(res.Text, `"truncated":true`) {
		t.Fatalf("playbook len %d, err %v", len(res.Text), err)
	}
}

func TestOversizedInputsFailSmallWithoutEcho(t *testing.T) {
	d := fullDeps()
	huge := strings.Repeat("Z", 200_000)
	for _, tc := range []struct{ field, val string }{
		{"effort", huge}, {"model", strings.Repeat("M", 300)}, {"dir", strings.Repeat("D", 5000)},
	} {
		args, _ := json.Marshal(map[string]string{"harness": "codex", tc.field: tc.val})
		_, err := toolByName(t, d, "apply").Handler(t.Context(), args)
		if err == nil || len(err.Error()) > 2048 || strings.Contains(err.Error(), tc.val[:20]) || !strings.Contains(err.Error(), tc.field) {
			t.Errorf("%s: err = %v", tc.field, err)
		}
	}
	// 4096 bytes of dir and 256 of model are accepted.
	args, _ := json.Marshal(map[string]string{"harness": "codex", "dir": strings.Repeat("d", 4096), "model": strings.Repeat("m", 256)})
	if _, err := toolByName(t, d, "apply").Handler(t.Context(), args); err != nil {
		t.Errorf("limits are inclusive: %v", err)
	}
}

func TestHugeNoteFailsAsTooLarge(t *testing.T) {
	d := fullDeps()
	d.Applier = fakeApplier{out: ApplyOutcome{Harness: "codex", Note: map[string]any{"k": strings.Repeat("N", 200_000)}}}
	_, err := toolByName(t, d, "apply").Handler(t.Context(), json.RawMessage(`{"harness":"codex"}`))
	if err == nil || len(err.Error()) > 2048 || !strings.Contains(err.Error(), "too large") || strings.Contains(err.Error(), "NNNN") {
		t.Fatalf("err = %v", err)
	}
}
