// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func routeEnv(t *testing.T, policy string) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	if policy != "" {
		writeFile(t, filepath.Join(dir, ".downshift", "policy.json"), policy)
	}
	return dir
}

func runRouteArgs(args ...string) (int, string, string) {
	var out, errw bytes.Buffer
	code := runRoute(args, &out, &errw)
	return code, out.String(), errw.String()
}

func TestRouteJSONAppliesDeclaredRule(t *testing.T) {
	dir := routeEnv(t, gatedRule)
	code, out, errs := runRouteArgs("--harness=claude-code", "--model="+frontierID, "--dir="+dir, "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, out)
	}
	want := map[string]any{
		"schema_version": float64(1), "harness": "claude-code",
		"requested_model": frontierID, "selected_model": smallID,
		"verdict": "rule", "applied": true,
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
	for _, k := range []string{"tier", "effort", "reason", "note"} {
		if _, ok := got[k]; !ok {
			t.Errorf("JSON lacks %q", k)
		}
	}
	if note, _ := got["note"].(map[string]any); note["rule_id"] != "cheap" {
		t.Errorf("note = %v, want rule_id cheap", note)
	}
}

func TestRouteTextAndTierHint(t *testing.T) {
	dir := routeEnv(t, "")
	code, out, errs := runRouteArgs("--harness=claude-code", "--model="+frontierID, "--tier=small", "--dir="+dir)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errs)
	}
	for _, want := range []string{"selected   " + smallID, "applied    true", "tier hint"} {
		if !strings.Contains(out, want) {
			t.Errorf("text lacks %q:\n%s", want, out)
		}
	}
}

func TestRouteNothingDeclaredKeeps(t *testing.T) {
	dir := routeEnv(t, "")
	code, out, _ := runRouteArgs("--harness=claude-code", "--model="+frontierID, "--effort=high", "--dir="+dir, "--json")
	var got routeJSON
	if err := json.Unmarshal([]byte(out), &got); code != 0 || err != nil {
		t.Fatalf("code %d err %v: %s", code, err, out)
	}
	if got.Applied || got.SelectedModel != frontierID || got.Effort != "high" {
		t.Errorf("got %+v, want the requested model and effort kept", got)
	}
}

func TestRouteUnknownModelIsKept(t *testing.T) {
	dir := routeEnv(t, gatedRule)
	code, out, _ := runRouteArgs("--harness=claude-code", "--model=no-such-model", "--dir="+dir)
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out, "selected   no-such-model") || !strings.Contains(out, "kept") {
		t.Errorf("unknown model not kept:\n%s", out)
	}
}

func TestRouteInvalidPolicyKeeps(t *testing.T) {
	dir := routeEnv(t, `{not json`)
	code, out, _ := runRouteArgs("--harness=claude-code", "--model="+frontierID, "--tier=small", "--dir="+dir)
	if code != 0 || !strings.Contains(out, "applied    false") {
		t.Errorf("invalid policy must keep (exit %d):\n%s", code, out)
	}
}

func TestRouteFlagErrors(t *testing.T) {
	routeEnv(t, "")
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing harness", nil, "--harness is required; valid ids: "},
		{"unknown harness", []string{"--harness=nope"}, `unknown harness "nope"`},
		{"bad tier", []string{"--harness=claude-code", "--tier=huge"}, "unknown tier"},
		{"bad effort", []string{"--harness=claude-code", "--effort=ludicrous"}, "unknown effort"},
		{"no prompt flag", []string{"--harness=claude-code", "--prompt=x"}, "flag provided but not defined"},
		{"stray argument", []string{"--harness=claude-code", "task text"}, "unexpected argument"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, out, errs := runRouteArgs(tc.args...)
			if code != 2 || out != "" || !strings.Contains(errs, tc.want) {
				t.Errorf("code %d out %q err %q, want exit 2 and %q", code, out, errs, tc.want)
			}
			if tc.name == "missing harness" && !strings.Contains(errs, "claude-code") {
				t.Errorf("error should list valid harness ids: %q", errs)
			}
		})
	}
}

func TestRouteHelpHasTheRequiredSections(t *testing.T) {
	text := routeHelp["route"]
	for _, s := range []string{"Usage:", "Flags:", "Examples:", "\nNext: "} {
		if !strings.Contains(text, s) {
			t.Errorf("route help lacks %q", strings.TrimSpace(s))
		}
	}
	if !strings.HasPrefix(text, "downshift route ") || strings.Contains(text, "--prompt") {
		t.Errorf("route help first line or prompt flag wrong: %q", text)
	}
}
