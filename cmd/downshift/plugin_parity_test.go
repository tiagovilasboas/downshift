// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The plugin's hooks/hooks.json must stay the exact plugin form of the hooks
// `setup` writes for the harness that declares a Plugin: same events, same
// matchers, same subcommands. Nothing else may live in the file.
func TestPluginHooksMatchSetupTarget(t *testing.T) {
	var target setupTarget
	for _, st := range setupTargets {
		if st.Plugin != "" {
			target = st
			break
		}
	}
	if target.Plugin == "" {
		t.Skip("no setup target declares a plugin")
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", "hooks", "hooks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("hooks.json: %v", err)
	}
	want := map[string]setupHook{}
	for _, h := range target.Hooks {
		want[h.Event+"|"+h.Matcher] = h
	}
	got := map[string]bool{}
	for event, entries := range file.Hooks {
		for _, e := range entries {
			key := event + "|" + e.Matcher
			spec, ok := want[key]
			if !ok {
				t.Errorf("hooks.json has %s matcher %q that setup does not write", event, e.Matcher)
				continue
			}
			if len(e.Hooks) != 1 || e.Hooks[0].Type != "command" {
				t.Errorf("%s: want exactly one command hook, got %+v", key, e.Hooks)
				continue
			}
			if cmd := e.Hooks[0].Command; cmd != "downshift "+spec.Args {
				t.Errorf("%s: command %q, want %q (bare binary on PATH, no logic)", key, cmd, "downshift "+spec.Args)
			}
			if strings.ContainsAny(e.Hooks[0].Command, "|;&$") {
				t.Errorf("%s: plugin hook must not carry shell logic: %q", key, e.Hooks[0].Command)
			}
			got[key] = true
		}
	}
	for key := range want {
		if !got[key] {
			t.Errorf("hooks.json is missing %s", key)
		}
	}
}
