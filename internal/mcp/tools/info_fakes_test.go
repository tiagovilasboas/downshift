// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/playbook"
)

var errPort = errors.New("port failed at /secret/path")

type fakeModels struct {
	list ModelList
	err  error
}

func (f fakeModels) Available(context.Context, string) (ModelList, error) { return f.list, f.err }

type fakePlaybooks struct {
	sets []playbook.Set
	err  error
}

func (f fakePlaybooks) Sets() ([]playbook.Set, error) { return f.sets, f.err }

type fakeStrain struct {
	entries []StrainEntry
	err     error
	got     *[]string
}

func (f fakeStrain) Strain(_ context.Context, ids []string) ([]StrainEntry, error) {
	if f.got != nil {
		*f.got = ids
	}
	return f.entries, f.err
}

type fakeApplier struct {
	out ApplyOutcome
	err error
	got *ApplyRequest
}

func (f fakeApplier) Apply(_ context.Context, in ApplyRequest) (ApplyOutcome, error) {
	if f.got != nil {
		*f.got = in
	}
	return f.out, f.err
}

func testSets() []playbook.Set {
	return []playbook.Set{
		{ID: "general", Name: "General", Text: "be brief", Source: playbook.SourceDefault},
		{ID: "backend", Name: "Backend", Text: "tests first\nsecond line", Source: playbook.SourceEdited},
	}
}

func fullDeps() Deps {
	return Deps{
		Models: fakeModels{list: ModelList{Harness: "claude-code", Models: []Model{
			{ID: "m1", Tier: "small", Source: "catalog"},
			{ID: "m2", Tier: "", Source: "discovered", Untiered: true},
		}, Fallback: true}},
		Playbooks: fakePlaybooks{sets: testSets()},
		Strain:    fakeStrain{entries: []StrainEntry{{Provider: "p", Window: "5h", UsedPct: 40, Headroom: "ok"}}},
		Applier:   fakeApplier{out: ApplyOutcome{Harness: "claude-code", SelectedModel: "m1", Verdict: "keep", Reason: "r"}},
		Harnesses: func() []string { return []string{"claude-code", "codex"} },
		Version:   "9.9.9",
	}
}

func toolByName(t *testing.T, d Deps, name string) Tool {
	t.Helper()
	for _, tl := range Info(d) {
		if tl.Name == name {
			return tl
		}
	}
	t.Fatalf("no tool %q", name)
	return Tool{}
}

// call runs a tool and decodes the envelope when it succeeds.
func call(t *testing.T, d Deps, name, args string) (map[string]json.RawMessage, error) {
	t.Helper()
	res, err := toolByName(t, d, name).Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		return nil, err
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(res.Text), &env); err != nil {
		t.Fatalf("envelope is not JSON: %v", err)
	}
	if _, ok := env["data"]; !ok {
		t.Fatalf("envelope has no data: %s", res.Text)
	}
	if _, ok := env["strain"]; !ok {
		t.Fatalf("envelope has no strain: %s", res.Text)
	}
	return env, nil
}

func mustFail(t *testing.T, d Deps, name, args string, want ...string) {
	t.Helper()
	_, err := call(t, d, name, args)
	if err == nil {
		t.Fatalf("%s %s: expected an error", name, args)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("error %q lacks %q", err, w)
		}
	}
	for _, bad := range []string{"/secret", "HOME", args} {
		if bad != "" && bad != "{}" && strings.Contains(err.Error(), bad) {
			t.Fatalf("error %q leaks %q", err, bad)
		}
	}
}
