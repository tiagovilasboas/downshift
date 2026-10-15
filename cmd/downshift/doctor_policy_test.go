// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestCheckMainPolicy(t *testing.T) {
	for _, tc := range []struct {
		doc     map[string]any
		detail  string
		wantFix bool
	}{
		{map[string]any{"model": "opusplan"}, "model opusplan", false},
		{map[string]any{"model": "claude-sonnet-5-5"}, "claude-sonnet-5-5 (pinned)", true},
		{map[string]any{}, "no model set", true},
	} {
		var got []string
		var fix string
		checkMainPolicy(func(id, status, detail, f string) {
			if status != doctorOK {
				t.Errorf("%v: status %s, want ok", tc.doc, status)
			}
			got = append(got, detail)
			fix = f
		}, "claude-code", tc.doc)
		if len(got) != 1 || !strings.Contains(got[0], tc.detail) || (fix != "") != tc.wantFix {
			t.Errorf("%v: got %v fix %q", tc.doc, got, fix)
		}
	}
	checkMainPolicy(func(id, _, _, _ string) { t.Errorf("cursor reported %s", id) }, "cursor", map[string]any{})
}
