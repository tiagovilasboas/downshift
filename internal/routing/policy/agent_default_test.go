// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import "testing"

// routing.agent_default_tier is on by default, a file turns it off, and the
// project's setting wins over the user's.
func TestAgentDefaultTier(t *testing.T) {
	cases := []struct {
		name          string
		project, user string
		want          bool
	}{
		{"no files: on", "", "", true},
		{"project off", `{"version":1,"routing":{"agent_default_tier":false}}`, "", false},
		{"user off", "", `{"version":1,"routing":{"agent_default_tier":false}}`, false},
		{"project on beats user off", `{"version":1,"routing":{"agent_default_tier":true}}`, `{"version":1,"routing":{"agent_default_tier":false}}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			loc := Locations{}
			if c.project != "" {
				loc.Project = projectFile(t, c.project)
			}
			if c.user != "" {
				loc.User = userFile(t, c.user)
			}
			p, err := Load(loc)
			if err != nil {
				t.Fatal(err)
			}
			if p.AgentDefaultRouting() != c.want {
				t.Fatalf("AgentDefaultRouting = %v, want %v", p.AgentDefaultRouting(), c.want)
			}
		})
	}
	if !(*Policy)(nil).AgentDefaultRouting() {
		t.Fatal("a nil policy is the default policy: the agent default must be on")
	}
	if _, err := Load(Locations{Project: projectFile(t, `{"version":1,"routing":{"agent_default_tier":"no"}}`)}); err == nil {
		t.Fatal("agent_default_tier must be a boolean")
	}
}
