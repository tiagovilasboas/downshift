// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

// overlapRows runs the real wiring match and the overlap check on a hook file,
// the way checkHarness does, and returns the rows it added.
func overlapRows(t *testing.T, harness, file string, verbose, viaPlugin bool) []bridgeRow {
	t.Helper()
	w, ok := doctorWiringFor(harness)
	if !ok {
		t.Fatalf("no wiring for %s", harness)
	}
	var raw any
	if err := json.Unmarshal([]byte(file), &raw); err != nil {
		t.Fatalf("fixture is not JSON: %v", err)
	}
	var rows []bridgeRow
	add := func(id, status, detail, fix string) { rows = append(rows, bridgeRow{id, status, detail, fix}) }
	checkHookOverlap(add, verbose, w, "SETTINGS", raw, viaPlugin)
	return rows
}

// Settings fixtures, nested shape (matcher group holding commands).
const (
	dsAgent = `{"matcher":"Agent|Task","hooks":[{"type":"command","command":"/b/downshift claude-code"}]}`
	dsBash  = `{"matcher":"Bash","hooks":[{"type":"command","command":"/b/downshift claude-code"}]}`
)

func pre(groups ...string) string {
	return `{"hooks":{"PreToolUse":[` + strings.Join(groups, ",") + `]}}`
}

func group(matcher, command string, args ...string) string {
	b, _ := json.Marshal(args)
	if len(args) == 0 {
		return `{"matcher":"` + matcher + `","hooks":[{"type":"command","command":` + jsonString(command) + `}]}`
	}
	return `{"matcher":"` + matcher + `","hooks":[{"type":"command","command":` + jsonString(command) + `,"args":` + string(b) + `}]}`
}

func jsonString(s string) string { b, _ := json.Marshal(s); return string(b) }

// The conflict is decided from the config alone: a recognised rewriter of the
// same kind, on the same event, with a matcher that can select the same tool.
func TestHookOverlapTable(t *testing.T) {
	rtkScript := group("Bash", "bash /home/u/.claude/hooks/rtk-rewrite.sh")
	cheap := group("Agent", "/usr/local/bin/python3", "/home/u/cheap-subagents/cheap_subagents.py")
	cases := []struct {
		name      string
		file      string
		verbose   bool
		viaPlugin bool
		wantNames []string // rewriter names expected in the warning rows, in order
		wantInfo  bool     // a verbose ok row about unrecognised hooks
		fixHas    []string // substrings of the first warning's fix
	}{
		{name: "downshift alone", file: pre(dsAgent, dsBash)},
		{name: "rtk script on Bash", file: pre(dsAgent, dsBash, rtkScript), wantNames: []string{"RTK"},
			fixHas: []string{"DOWNSHIFT_EXEC=0", `"exec": false`, `remove the RTK entry under "PreToolUse" in SETTINGS`}},
		{name: "rtk native hook command", file: pre(dsBash, group("Bash", "rtk hook claude")), wantNames: []string{"RTK"}},
		{name: "rtk windows path and exe", file: pre(dsBash, group("Bash", `C:\Users\u\.cargo\bin\rtk.exe hook claude`)), wantNames: []string{"RTK"}},
		{name: "rtk quoted path with spaces", file: pre(dsBash, group("Bash", `"C:\\Program Files\\rtk\\rtk.exe" hook claude`)), wantNames: []string{"RTK"}},
		{name: "rtk with empty matcher selects every tool", file: pre(dsBash, group("", "rtk hook claude")), wantNames: []string{"RTK"}},
		{name: "rtk with star matcher", file: pre(dsBash, group("*", "rtk hook claude")), wantNames: []string{"RTK"}},
		{name: "rtk behind an interpreter with flags", file: pre(dsBash, group("Bash", "bash -e /h/rtk-rewrite.sh")), wantNames: []string{"RTK"}},
		{name: "rtk anchored regex matcher", file: pre(dsBash, group("^Bash$", "rtk hook claude")), wantNames: []string{"RTK"}},
		{name: "rtk on Edit does not overlap", file: pre(dsBash, group("Edit|Write", "rtk hook claude"))},
		{name: "rtk on another event does not overlap", file: `{"hooks":{"PreToolUse":[` + dsBash + `],"PostToolUse":[` + group("Bash", "rtk hook claude") + `]}}`},
		{name: "rtk without downshift's shell hook has nothing to lose", file: pre(dsAgent, rtkScript)},
		{name: "rtk shell rewriter on the model hook's tool is a different field", file: pre(dsAgent, group("Agent", "rtk hook claude"))},
		{name: "cheap-subagents exec form", file: pre(dsAgent, dsBash, cheap), wantNames: []string{"cheap-subagents"},
			fixHas: []string{`remove the cheap-subagents entry under "PreToolUse" in SETTINGS`, "declared policy"}},
		{name: "cheap-subagents behind env and python", file: pre(dsAgent, group("Agent", "/usr/bin/env", "python3", "-u", "/h/cheap_subagents.py")), wantNames: []string{"cheap-subagents"}},
		{name: "cheap-subagents on Bash is outside its tool", file: pre(dsAgent, group("Bash", "/usr/bin/python3", "/h/cheap_subagents.py"))},
		{name: "both companions", file: pre(dsAgent, dsBash, cheap, rtkScript), wantNames: []string{"cheap-subagents", "RTK"}},
		{name: "same rewriter in two entries is reported once", file: pre(dsBash, rtkScript, group("Bash", "rtk hook claude")), wantNames: []string{"RTK"}},
		{name: "plugin carries downshift's hooks", file: pre(rtkScript), viaPlugin: true, wantNames: []string{"RTK"}},
		{name: "unrecognised hook is silent", file: pre(dsBash, group("Bash", "/opt/audit/log-bash.sh"))},
		{name: "unrecognised hook is counted in verbose", file: pre(dsBash, group("Bash", "/opt/audit/log-bash.sh")), verbose: true, wantInfo: true},
		{name: "a tool named like rtk in an argument is not rtk", file: pre(dsBash, group("Bash", "/opt/audit/log.sh", "rtk")), wantInfo: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows := overlapRows(t, "claude-code", c.file, c.verbose, c.viaPlugin)
			var warns, infos []bridgeRow
			for _, r := range rows {
				if r.id != "hook overlap" {
					t.Fatalf("unexpected row id %q", r.id)
				}
				if r.status == doctorWarn {
					warns = append(warns, r)
				} else {
					infos = append(infos, r)
				}
			}
			if len(warns) != len(c.wantNames) {
				t.Fatalf("got %d warning(s) %+v, want %v", len(warns), warns, c.wantNames)
			}
			for i, name := range c.wantNames {
				if !strings.HasPrefix(warns[i].detail, name+" also rewrites") {
					t.Errorf("warning %d detail = %q, want it to start with %q", i, warns[i].detail, name)
				}
				if warns[i].fix == "" {
					t.Errorf("warning %d has no fix", i)
				}
				if n := len([]rune(warns[i].detail)); n > 62 {
					t.Errorf("warning %d detail is %d runes; text mode cuts rows near 60: %q", i, n, warns[i].detail)
				}
			}
			for _, want := range c.fixHas {
				if len(warns) == 0 || !strings.Contains(warns[0].fix, want) {
					t.Errorf("fix %q lacks %q", warns, want)
				}
			}
			if c.wantInfo != (len(infos) == 1 && infos[0].status == doctorOK) {
				t.Errorf("verbose info rows = %+v, want info=%v", infos, c.wantInfo)
			}
		})
	}
}

// The check names no harness: the same function covers a harness whose file
// uses flat entries, a different event spelling and different matchers.
func TestHookOverlapOtherHarnessShapes(t *testing.T) {
	cursor := `{"version":1,"hooks":{"preToolUse":[
	  {"matcher":"Task","command":"/b/downshift cursor"},
	  {"matcher":"Shell","command":"/b/downshift cursor"},
	  {"matcher":"Shell","command":"rtk hook cursor"}]}}`
	rows := overlapRows(t, "cursor", cursor, false, false)
	if len(rows) != 1 || rows[0].status != doctorWarn || !strings.HasPrefix(rows[0].detail, "RTK also rewrites the command;") {
		t.Fatalf("cursor rows = %+v", rows)
	}

	codex := `{"hooks":{"PreToolUse":[
	  {"matcher":"(^Agent$|.*spawn_agent$)","hooks":[{"type":"command","command":"/b/downshift codex"}]},
	  {"matcher":"Agent","hooks":[{"type":"command","command":"python3","args":["/h/cheap_subagents.py"]}]}]}}`
	rows = overlapRows(t, "codex", codex, false, false)
	if len(rows) != 1 || !strings.HasPrefix(rows[0].detail, "cheap-subagents also rewrites the model;") {
		t.Fatalf("codex rows = %+v", rows)
	}
}

func TestMatchersOverlap(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"Bash", "Bash", true},
		{"Bash", "^Bash$", true},
		{"Bash", "Bash.*", true},
		{"Bash", "Edit|Write", false},
		{"Agent|Task", "Agent", true},
		{"Agent|Task", "Task", true},
		{"Agent|Task", "Agent|Bash", true},
		{"Agent|Task", "Bash", false},
		{"Bash", "", true},
		{"Bash", "*", true},
		{"Bash", ".*", true},
		{"(^Agent$|.*spawn_agent$)", "Agent", true},
		{"(^Agent$|.*spawn_agent$)", "mcp_spawn_agent", true},
		{"(^Agent$|.*spawn_agent$)", "Bash", false},
		{"^Bash$", "Bash", true},
		{"Bash", "[unclosed", false},
		{"*", "Task", true},
	} {
		if got := matchersOverlap(c.a, c.b); got != c.want {
			t.Errorf("matchersOverlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
		if got := matchersOverlap(c.b, c.a); got != c.want {
			t.Errorf("matchersOverlap(%q, %q) = %v, want %v (symmetry)", c.b, c.a, got, c.want)
		}
	}
}

// Every Downshift hook that can rewrite input is tagged, and only those: a tag
// on an observe-only hook would make doctor warn about harmless neighbours.
func TestRewritesTagsInTargetTable(t *testing.T) {
	want := map[string]string{ // harness|event|matcher -> kind
		"claude-code|PreToolUse|Agent|Task":         setup.RewritesModel,
		"claude-code|PreToolUse|Bash":               setup.RewritesShell,
		"cursor|preToolUse|Task":                    setup.RewritesModel,
		"cursor|preToolUse|Shell":                   setup.RewritesShell,
		"codex|PreToolUse|(^Agent$|.*spawn_agent$)": setup.RewritesModel,
		"codex|PreToolUse|^Bash$":                   setup.RewritesShell,
		"antigravity|PreToolUse|invoke_subagent":    setup.RewritesModel,
		"antigravity|PreToolUse|run_command":        setup.RewritesShell,
	}
	got := map[string]string{}
	for _, tg := range setup.Targets {
		for _, h := range tg.Hooks {
			if h.Rewrites != "" {
				got[tg.Name+"|"+h.Event+"|"+h.Matcher] = h.Rewrites
			}
		}
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: Rewrites = %q, want %q", k, got[k], v)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s is tagged as a rewriter but is not in the expected list", k)
		}
	}
}

// End to end through buildDoctorReport: a real settings file with Downshift's
// hooks plus RTK on Bash and cheap-subagents on Agent yields two warnings that
// carry a fix, and removing the companions clears them.
func TestDoctorReportsHookOverlap(t *testing.T) {
	f := newDoctorFixture(t)
	useSessionList(t, "claude-code", "claude-haiku-4-5", "claude-sonnet-4-5", "claude-opus-4-5")
	bin := jsonString(f.bin + " claude-code")
	ds := func(matcher string) string {
		return `{"matcher":"` + matcher + `","hooks":[{"type":"command","command":` + bin + `}]}`
	}
	settings := func(extra ...string) string {
		return `{"hooks":{"PreToolUse":[` + strings.Join(append([]string{ds("Agent|Task"), ds("Bash")}, extra...), ",") + `]}}`
	}
	count := func(rep *doctorReport) (n int) {
		for _, c := range rep.Checks {
			if c.ID == "hook overlap" {
				n++
				wantStatus(t, c, doctorWarn)
			}
		}
		return n
	}

	path := f.home + "/.claude/settings.json"
	writeFile(t, path, settings())
	if n := count(buildDoctorReport(f.env)); n != 0 {
		t.Fatalf("clean file: %d overlap rows", n)
	}
	writeFile(t, path, settings(group("Bash", "bash /h/rtk-rewrite.sh"), group("Agent", "/usr/bin/python3", "/h/cheap_subagents.py")))
	rep := buildDoctorReport(f.env)
	if n := count(rep); n != 2 {
		t.Fatalf("with both companions: %d overlap rows, want 2: %+v", n, rep.Checks)
	}
	if rep.Warnings < 2 {
		t.Errorf("report warnings = %d, want the overlaps counted", rep.Warnings)
	}
}
