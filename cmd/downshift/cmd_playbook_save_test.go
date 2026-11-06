// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/playbook"
)

func runSave(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runPlaybookIn(append([]string{"save"}, args...), strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func TestPlaybookSaveCreatesAndReplaces(t *testing.T) {
	playbookEnv(t)
	dir, _ := playbook.Dir()
	if code, _, e := runSave(t, "Rule one.\n", "Mobile App"); code != 0 {
		t.Fatalf("code %d: %s", code, e)
	}
	sets, _ := playbook.Load(dir)
	if s, ok := playbook.Find(sets, "mobile app"); !ok || s.Text != "Rule one." || s.ID != "mobile-app" {
		t.Fatalf("created = %+v %v", s, ok)
	}
	if code, _, e := runSave(t, "Replaced.", "backend"); code != 0 {
		t.Fatalf("code %d: %s", code, e)
	}
	sets, _ = playbook.Load(dir)
	if s, _ := playbook.Find(sets, "backend"); s.Text != "Replaced." || s.ID != "backend" {
		t.Errorf("backend = %+v", s)
	}
}

func TestPlaybookSaveRefusals(t *testing.T) {
	playbookEnv(t)
	cases := []struct {
		name  string
		stdin string
		args  []string
		code  int
	}{
		{"empty input", "  \n", []string{"x"}, 1},
		{"invalid name", "text", []string{"!!!"}, 1},
		{"no name", "text", nil, 2},
	}
	for _, c := range cases {
		if code, _, _ := runSave(t, c.stdin, c.args...); code != c.code {
			t.Errorf("%s: code %d, want %d", c.name, code, c.code)
		}
	}
}
