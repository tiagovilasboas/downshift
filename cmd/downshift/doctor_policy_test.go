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
		{"both set", map[string]any{"model": "opusplan", "permissions": map[string]any{"defaultMode": "plan"}},
			[]string{"model opusplan", "permissions.defaultMode plan"}},
		{"pinned by the user", map[string]any{"model": "claude-sonnet-5-5", "permissions": map[string]any{"defaultMode": "acceptEdits"}},
			[]string{"claude-sonnet-5-5 (set by you)", "acceptEdits (set by you)"}},
		{"nothing set", map[string]any{}, []string{"no model set", "no permissions.defaultMode set"}},
		{"permissions without a mode", map[string]any{"permissions": map[string]any{"allow": []any{}}},
			[]string{"no model set", "no permissions.defaultMode set"}},
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
