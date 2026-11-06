// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

type fakeHistory struct {
	sum SessionSummary
	err error
	got *string
}

func (f fakeHistory) Summary(_ context.Context, harness string) (SessionSummary, error) {
	if f.got != nil {
		*f.got = harness
	}
	return f.sum, f.err
}

func harnessDeps() Deps {
	d := fullDeps()
	d.Harnesses = func() []string {
		var ids []string
		for _, n := range core.AllHarnessCaps() {
			ids = append(ids, n.Harness)
		}
		return ids
	}
	return d
}

func providersSession(t *testing.T, d Deps, harness string) *SessionSummary {
	t.Helper()
	env, err := call(t, d, "providers", `{"harness":"`+harness+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	var got providersData
	if err := json.Unmarshal(env["data"], &got); err != nil {
		t.Fatal(err)
	}
	return got.Session
}

// providers carries the session's spawn pattern for every harness, and says
// when the session cannot be known instead of guessing one.
func TestProvidersSessionSummaryForEveryHarness(t *testing.T) {
	known := SessionSummary{Known: true, Spawns: 3, ByTier: map[string]int{"mid": 2, "small": 1},
		ByAgentType: map[string]int{"custom": 3}, AboveAgentDefault: 1}
	unknown := SessionSummary{Note: "no session id", ByTier: map[string]int{}, ByAgentType: map[string]int{}}
	for _, n := range core.AllHarnessCaps() {
		t.Run(n.Harness, func(t *testing.T) {
			var asked string
			d := harnessDeps()
			d.History = fakeHistory{sum: known, got: &asked}
			s := providersSession(t, d, n.Harness)
			if asked != n.Harness || s == nil || !s.Known || s.Spawns != 3 || s.ByTier["mid"] != 2 || s.AboveAgentDefault != 1 {
				t.Fatalf("asked %q, session %+v", asked, s)
			}
			d.History = fakeHistory{sum: unknown}
			if s := providersSession(t, d, n.Harness); s == nil || s.Known || s.Note == "" || s.Spawns != 0 {
				t.Fatalf("unknown session = %+v", s)
			}
		})
	}
}

// A failing or missing history port drops the block and never fails providers.
func TestProvidersSessionSummaryOptional(t *testing.T) {
	d := fullDeps()
	if s := providersSession(t, d, "claude-code"); s != nil {
		t.Fatalf("no port: session = %+v, want none", s)
	}
	d.History = fakeHistory{err: errors.New("boom at /secret")}
	if s := providersSession(t, d, "claude-code"); s != nil {
		t.Fatalf("failing port: session = %+v, want none", s)
	}
}
