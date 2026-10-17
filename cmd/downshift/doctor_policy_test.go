// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestCheckMainPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  map[string]any
		want []string // one substring per reported line, in order
	}{
		{"all set", map[string]any{"model": "opusplan", "permissions": map[string]any{"defaultMode": "plan"}, "enabledPlugins": map[string]any{"cc-plugin-you-should-know@builtin": true}},
			[]string{"model opusplan", "permissions.defaultMode plan", "you-should-know on (built-in Claude Code mod; each check is a billed request; /plugin disable cc-plugin-you-should-know@builtin to turn off)"}},
		{"pinned by the user", map[string]any{"model": "claude-sonnet-5-5", "permissions": map[string]any{"defaultMode": "acceptEdits"}, "enabledPlugins": map[string]any{"downshift@downshift": true, "cc-plugin-you-should-know@builtin": false}},
			[]string{"claude-sonnet-5-5 (set by you)", "acceptEdits (set by you)", "you-should-know false (set by you)"}},
		{"nothing set", map[string]any{}, []string{"no model set", "no permissions.defaultMode set", "no you-should-know set"}},
		{"permissions without a mode", map[string]any{"permissions": map[string]any{"allow": []any{}}},
			[]string{"no model set", "no permissions.defaultMode set", "no you-should-know set"}},
	} {
		var got []string
		checkMainPolicy(func(id, status, detail, _ string) {
			if status != doctorOK {
				t.Errorf("%s: status %s, want ok", tc.name, status)
			}
			got = append(got, detail)
		}, "claude-code", tc.doc)
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %d lines", tc.name, got, len(tc.want))
			continue
		}
		for i, w := range tc.want {
			if !strings.Contains(got[i], w) {
				t.Errorf("%s: line %d = %q, want it to contain %q", tc.name, i, got[i], w)
			}
		}
	}
	checkMainPolicy(func(id, _, _, _ string) { t.Errorf("cursor reported %s", id) }, "cursor", map[string]any{})
}
