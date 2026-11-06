// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"reflect"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	if s, err := LoadState(); err != nil || s != (State{}) {
		t.Fatalf("missing file = %+v, %v; want zero", s, err)
	}
	want := State{Enabled: true, Name: "9router", Endpoint: "http://localhost:20128"}
	if err := SaveState(want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState()
	if err != nil || got != want {
		t.Fatalf("got %+v, %v; want %+v", got, err, want)
	}
}

func TestWorkerEnv(t *testing.T) {
	on := func(ep string) State { return State{Enabled: true, Name: "9router", Endpoint: ep} }
	cases := []struct {
		name  string
		state State
		agent string
		want  []string
	}{
		{"claude", on("http://localhost:20128"), "claude", []string{"ANTHROPIC_BASE_URL=http://localhost:20128"}},
		{"codex", on("http://localhost:20128"), "codex", []string{"OPENAI_BASE_URL=http://localhost:20128/v1"}},
		{"disabled", State{Name: "9router", Endpoint: "http://localhost:20128"}, "claude", nil},
		{"trailing slash", on("http://localhost:20128/"), "codex", []string{"OPENAI_BASE_URL=http://localhost:20128/v1"}},
		{"v1 present claude", on("http://localhost:20128/v1"), "claude", []string{"ANTHROPIC_BASE_URL=http://localhost:20128"}},
		{"v1 present codex", on("http://localhost:20128/v1/"), "codex", []string{"OPENAI_BASE_URL=http://localhost:20128/v1"}},
		{"unknown agent", on("http://localhost:20128"), "other", nil},
		{"empty endpoint", on(""), "claude", nil},
	}
	for _, c := range cases {
		if got := WorkerEnv(c.state, c.agent); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestLoadStateRetiredGatewayReadsOff(t *testing.T) {
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	if err := SaveState(State{Enabled: true, Name: "omniroute", Endpoint: "http://localhost:20128/v1"}); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState()
	if err != nil {
		t.Fatal(err)
	}
	if got.Enabled || !got.Retired || got.Name != "omniroute" {
		t.Fatalf("got %+v; want off, retired, name kept", got)
	}
	if env := WorkerEnv(got, "claude"); env != nil {
		t.Fatalf("WorkerEnv = %v; want nil for a retired gateway", env)
	}
}
