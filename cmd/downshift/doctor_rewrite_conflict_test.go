// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

// claudeCodeWiring returns the claude-code row of the real target table and caps, so
// the test reads the same data doctor does.
func claudeCodeWiring(t *testing.T) (core.NamedCaps, doctorWiring) {
	t.Helper()
	for _, nc := range core.AllHarnessCaps() {
		if nc.Harness == "claude-code" {
			w, ok := doctorWiringFor(nc.Harness)
			if !ok {
				t.Fatal("no wiring for claude-code")
			}
			return nc, w
		}
	}
	t.Fatal("claude-code not in AllHarnessCaps")
	return core.NamedCaps{}, doctorWiring{}
}

func TestCheckRewriteConflicts(t *testing.T) {
	nc, w := claudeCodeWiring(t)
	rtk := `{"matcher":"Bash","hooks":[{"type":"command","command":"rtk hook claude"}]}`
	cheap := `{"matcher":"Agent","hooks":[{"type":"command","command":"python3","args":["/h/.claude/cheap_subagents.py"]}]}`
	tests := []struct {
		name    string
		pre     string // the PreToolUse array body
		post    string // the PostToolUse array body
		execOff bool
		want    []string // names expected in the warnings, in order
		tools   []string // tool list expected in the same warnings
	}{
		{name: "no other hooks", pre: ``},
		{name: "RTK on Bash", pre: rtk, want: []string{"RTK"}, tools: []string{"Bash"}},
		{name: "RTK by absolute path", pre: `{"matcher":"Bash","hooks":[{"type":"command","command":"/home/u/.cargo/bin/rtk hook claude"}]}`, want: []string{"RTK"}, tools: []string{"Bash"}},
		{name: "RTK legacy script", pre: `{"matcher":"Bash","hooks":[{"type":"command","command":"/h/.claude/hooks/rtk-rewrite.sh"}]}`, want: []string{"RTK"}, tools: []string{"Bash"}},
		{name: "RTK on Bash with the compact wrapper off", pre: rtk, execOff: true},
		{name: "RTK on a tool Downshift does not rewrite", pre: `{"matcher":"Read","hooks":[{"type":"command","command":"rtk hook claude"}]}`},
		{name: "RTK registered only for PostToolUse", pre: ``, post: rtk},
		{name: "cheap-subagents on Agent", pre: cheap, want: []string{"cheap-subagents"}, tools: []string{"Agent"}},
		{name: "cheap-subagents with the wrapper off still conflicts", pre: cheap, execOff: true, want: []string{"cheap-subagents"}, tools: []string{"Agent"}},
		{name: "both", pre: rtk + "," + cheap, want: []string{"cheap-subagents", "RTK"}, tools: []string{"Agent", "Bash"}}, // Downshift's hook order: Agent|Task, then Bash
		{name: "a match-all matcher selects the Downshift tools", pre: `{"matcher":"","hooks":[{"type":"command","command":"rtk hook claude"}]}`, want: []string{"RTK", "RTK"}, tools: []string{"Agent/Task", "Bash"}},
		{name: "regex matcher that covers Bash", pre: `{"matcher":"^(Bash|Read)$","hooks":[{"type":"command","command":"rtk hook claude"}]}`, want: []string{"RTK"}, tools: []string{"Bash"}},
		{name: "an unrelated hook on Bash", pre: `{"matcher":"Bash","hooks":[{"type":"command","command":"/usr/local/bin/audit-log"}]}`},
		{name: "a script that only mentions rtk", pre: `{"matcher":"Bash","hooks":[{"type":"command","command":"echo rtk is great"}]}`},
		{name: "Downshift's own hook", pre: `{"matcher":"Bash","hooks":[{"type":"command","command":"/usr/local/bin/downshift claude-code"}]}`},
		{name: "flat entry shape", pre: `{"matcher":"Bash","command":"rtk hook claude"}`, want: []string{"RTK"}, tools: []string{"Bash"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			doc := `{"hooks":{"PreToolUse":[` + tc.pre + `],"PostToolUse":[` + tc.post + `]}}`
			var raw any
			if err := json.Unmarshal([]byte(doc), &raw); err != nil {
				t.Fatal(err)
			}
			var got []doctorCheck
			add := func(id, status, detail, fix string) {
				got = append(got, doctorCheck{ID: id, Status: status, Detail: detail, Fix: fix})
			}
			checkRewriteConflicts(add, nc, w, raw, "~/.claude/settings.json", !tc.execOff)
			if len(got) != len(tc.want) {
				t.Fatalf("got %d checks %+v, want %d (%v)", len(got), got, len(tc.want), tc.want)
			}
			for i, c := range got {
				wantStatus(t, c, doctorWarn)
				if c.ID != "hook conflict" || !strings.HasPrefix(c.Detail, tc.want[i]+" also rewrites "+tc.tools[i]+" input") {
					t.Errorf("check %d = %q / %q, want %s on %s", i, c.ID, c.Detail, tc.want[i], tc.tools[i])
				}
				if !strings.Contains(c.Detail, "settings.json") || !strings.Contains(c.Fix, tc.want[i]) {
					t.Errorf("check %d should name the file and the product to remove: %q / %q", i, c.Detail, c.Fix)
				}
			}
		})
	}
}

// A harness that cannot rewrite at all gets no conflict warning, whatever its
// hook file holds (KiroCrew answers only allow or block).
func TestCheckRewriteConflicts_HarnessThatCannotRewrite(t *testing.T) {
	var raw any
	_ = json.Unmarshal([]byte(`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"command":"rtk hook claude"}]}]}}`), &raw)
	nc, w := claudeCodeWiring(t)
	nc.Caps.CanRewriteModel, nc.Caps.CanRewriteShellCommand = false, false
	called := false
	checkRewriteConflicts(func(string, string, string, string) { called = true }, nc, w, raw, "f", true)
	if called {
		t.Fatal("a harness without rewrite capability must not warn")
	}
}

// The check runs inside the full doctor report, from the real target table, so
// a user who installed Downshift as a plugin (no hooks in settings.json) and
// RTK in settings.json still sees the warning.
func TestDoctor_RewriteConflictInReport(t *testing.T) {
	f := newDoctorFixture(t)
	old := execConfigEnabled
	execConfigEnabled = func() bool { return true }
	t.Cleanup(func() { execConfigEnabled = old })
	settings := strings.Replace(claudeSettings(f.bin), `{"hooks":{"PreToolUse":[`,
		`{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"rtk hook claude"}]},`, 1)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), settings)

	c := f.check(t, buildDoctorReport(f.env), "claude-code", "hook conflict")
	wantStatus(t, c, doctorWarn)
	if !strings.HasPrefix(c.Detail, "RTK also rewrites Bash input") {
		t.Errorf("detail %q", c.Detail)
	}
	// Turning the wrapper off in the config is one of the two offered fixes: it must clear the warning.
	execConfigEnabled = func() bool { return false }
	rep := buildDoctorReport(f.env)
	if _, ok := findCheck(rep, "claude-code", "hook conflict"); ok {
		t.Error(`"exec": false should clear the RTK conflict`)
	}
}

func TestRewriterTargetsStayData(t *testing.T) {
	// The conflict check relies on the first-class rewriting hooks carrying the
	// harness name as their subcommand; fail loudly if the table changes shape.
	for _, tg := range setup.Targets {
		if tg.Name != "claude-code" {
			continue
		}
		n := 0
		for _, h := range tg.Hooks {
			if h.Args == tg.Name {
				n++
			}
		}
		if n < 2 {
			t.Fatalf("claude-code should have a model and a shell rewriting hook named %q, found %d", tg.Name, n)
		}
	}
}
