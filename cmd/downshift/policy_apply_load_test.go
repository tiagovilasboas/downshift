// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// A policy that does not load is not applied, not even partly. The load error is
// shown as it is and nothing is written, including the valid profile beside the
// bad one.
func TestPolicyApply_InvalidPolicyIsRefusedAndWritesNothing(t *testing.T) {
	good := profileJSON("good", "small", "A valid profile")
	cases := []struct {
		name    string
		project string
		user    string // user policy, "" for none
		want    string // substring of the load error
	}{
		{"not JSON", `{"version":1,`, "", "not valid JSON"},
		{"unknown routing key", `{"version":1,"routing":{"profiles":[` + good + `],"bogus":1}}`, "", "unknown field"},
		{"unknown profile key", `{"version":1,"routing":{"profiles":[` + good + `,{"name":"p","tier":"small","harness":"claude-code","model":"x"}]}}`, "", "unknown field"},
		{"path traversal in a name", routingPolicy(good, profileJSON("../evil", "small", "x")), "", "not a valid name"},
		{"slash in a name", routingPolicy(good, profileJSON("sub/agent", "small", "x")), "", "not a valid name"},
		{"unknown tier", routingPolicy(good, profileJSON("p", "strong", "x")), "", "small, mid or frontier"},
		{"missing harness", routingPolicy(good, `{"name":"p","tier":"small"}`), "", "unknown harness"},
		{"duplicate profile", routingPolicy(good, good), "", "duplicate profile"},
		{"routing without version", `{"routing":{"profiles":[` + good + `]}}`, "", "must be 1"},
		{"user policy invalid", routingPolicy(good), `{"version":1,"routing":{"profiles":[{"name":"Bad","tier":"small","harness":"claude-code"}]}}`, "not a valid name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, c.project)
			if c.user != "" {
				writeTestFile(t, filepath.Join(home, ".downshift", "policy.json"), c.user, 0o644)
			}
			code, stdout, stderr := runApply(t, dir, home)
			if code != 1 {
				t.Fatalf("exit %d, want 1\nstdout: %s\nstderr: %s", code, stdout, stderr)
			}
			if !strings.Contains(stderr, "policy:") || !strings.Contains(stderr, c.want) {
				t.Errorf("stderr lacks the load error %q:\n%s", c.want, stderr)
			}
			if !missing(t, filepath.Join(dir, ".claude")) {
				t.Error("something was written for a policy that did not load")
			}
		})
	}
}

// Profiles belong under routing.profiles. A top-level "profiles" key is not part
// of the schema and is not read: nothing is applied, and the message says where
// profiles go.
func TestPolicyApply_TopLevelProfilesKeyIsNotRead(t *testing.T) {
	cases := []struct{ name, policy string }{
		{"top-level profiles", `{"version":1,"profiles":[{"name":"scout","tier":"small","harness":"claude-code","description":"x"}]}`},
		{"routing with no profiles", `{"version":1,"routing":{}}`},
		{"no routing at all", `{"version":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, home := setupPolicyApplyEnv(t, c.policy)
			code, stdout, stderr := runApply(t, dir, home)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, stderr)
			}
			if !strings.Contains(stdout, "no routing.profiles entry for claude-code") {
				t.Errorf("output does not say there is nothing to apply:\n%s", stdout)
			}
			if !missing(t, filepath.Join(dir, ".claude")) {
				t.Error("an agent was written from a key outside the schema")
			}
		})
	}
}

// The canonical shape: profiles under routing, next to rules, with the keys the
// schema declares. The project profile wins over the user's of the same name,
// and a user-only profile is applied too, under the project root.
func TestPolicyApply_ReadsCanonicalRoutingProfiles(t *testing.T) {
	project := `{"version":1,"routing":{
	  "critical_threshold":90,
	  "profiles":[
	    {"name":"reviewer","tier":"mid","harness":"claude-code","fallback":"careful","description":"Reviews diffs"},
	    {"name":"careful","tier":"small","harness":"claude-code","description":"Careful work"},
	    {"name":"codexonly","tier":"mid","harness":"codex"}],
	  "rules":[{"id":"docs","kinds":["docs"],"profile":"careful"}]
	}}`
	user := `{"version":1,"routing":{"profiles":[
	  {"name":"reviewer","tier":"frontier","harness":"claude-code","description":"User version"},
	  {"name":"helper","tier":"mid","harness":"claude-code","description":"From the user policy"}]}}`
	dir, home := setupPolicyApplyEnv(t, project)
	writeTestFile(t, filepath.Join(home, ".downshift", "policy.json"), user, 0o644)

	if code, _, stderr := runApply(t, dir, home); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	agents := filepath.Join(dir, ".claude", "agents")
	want := map[string]core.Tier{"reviewer": core.TierMid, "careful": core.TierSmall, "helper": core.TierMid}
	for name, tier := range want {
		front, _ := splitAgent(t, readTestFile(t, filepath.Join(agents, name+".md")))
		if front["model"] != tierModel(tier) {
			t.Errorf("%s: model = %q, want %q", name, front["model"], tierModel(tier))
		}
	}
	if !strings.Contains(readTestFile(t, filepath.Join(agents, "reviewer.md")), "Reviews diffs") {
		t.Error("reviewer did not take the project policy's description")
	}
	if !missing(t, filepath.Join(agents, "codexonly.md")) {
		t.Error("a codex profile was written as a Claude Code agent")
	}
}
