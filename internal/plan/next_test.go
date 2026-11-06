// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"strings"
	"testing"
)

func buildNext(t *testing.T, gw Gateway, w Workers, parts ...Part) Plan {
	t.Helper()
	p, errs := Build(Input{Goal: "g", Parts: parts}, gw, w, shippedIDs)
	if errs != nil {
		t.Fatal(errs)
	}
	return p
}

func TestNextStepsTierHint(t *testing.T) {
	cases := []struct {
		agent string
		hint  bool
	}{{"researcher", true}, {"coder", false}, {"architect", false}}
	for _, c := range cases {
		t.Run(c.agent, func(t *testing.T) {
			s := buildNext(t, Gateway{}, Workers{}, part("x", c.agent, 1)).Next[0]
			if s.Kind != KindSpawn || s.SubagentType != "downshift:"+c.agent {
				t.Fatalf("step = %+v", s)
			}
			if got := strings.HasPrefix(s.Prompt, "[tier=small] "); got != c.hint {
				t.Errorf("hint = %v for tier %s, prompt %q", got, s.Tier, s.Prompt)
			}
		})
	}
}

func TestNextStepsWorkersAndRoute(t *testing.T) {
	cases := []struct {
		name  string
		gw    Gateway
		route string
	}{{"off", Gateway{}, "direct"}, {"on", Gateway{Enabled: true, Name: "9router"}, "9router"}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := Part{ID: "w", Title: "Scan", Agent: "researcher", Points: 1, Worker: "claude"}
			p := buildNext(t, c.gw, Workers{Mode: WorkersWrite, Agents: []string{"claude"}}, w)
			s := p.Next[0]
			if s.Kind != KindDelegate || s.Agent != "claude" || s.SubagentType != "" || s.Route != c.route {
				t.Fatalf("step = %+v", s)
			}
			if strings.Contains(s.Prompt, "[tier=small]") || !strings.Contains(s.Prompt, "Scan") {
				t.Errorf("prompt %q", s.Prompt)
			}
			md := p.Markdown()
			for _, want := range []string{"## Next steps", `"agent":"claude"`, `"mode":"read"`, "route " + c.route} {
				if !strings.Contains(md, want) {
					t.Errorf("markdown lacks %q:\n%s", want, md)
				}
			}
		})
	}
}

func TestNextStepsWavesAndPrompt(t *testing.T) {
	p := buildNext(t, Gateway{}, Workers{},
		Part{ID: "b", Title: "B", Agent: "tester", Points: 2, DependsOn: []string{"a"}, Files: []string{"b_test.go"}},
		part("a", "coder", 3),
		part("c", "coder", 1),
	)
	var order []string
	for _, s := range p.Next {
		order = append(order, s.Part)
	}
	if strings.Join(order, ",") != "a,c,b" || p.Next[0].Wave != 1 || p.Next[2].Wave != 2 {
		t.Errorf("order %v", order)
	}
	pr := p.Next[2].Prompt
	for _, want := range []string{"tdd-verification", "Task: B", "Files: b_test.go", "Depends on: a"} {
		if !strings.Contains(pr, want) {
			t.Errorf("prompt %q lacks %q", pr, want)
		}
	}
	md := p.Markdown()
	if strings.Count(md, "Wave 1 (") != 1 || strings.Count(md, "Wave 2 (") != 1 || strings.Contains(md, "DOWNSHIFT_WORKERS") {
		t.Errorf("markdown:\n%s", md)
	}
}

func TestNextStepsByWorkerMode(t *testing.T) {
	read := Part{ID: "r", Title: "R", Agent: "researcher", Points: 1, Worker: "claude"}
	edit := Part{ID: "e", Title: "E", Agent: "coder", Points: 1, Worker: "claude"}
	native := part("n", "coder", 1)
	off := Workers{}
	rd := Workers{Mode: WorkersRead, Agents: []string{"claude"}}
	wr := Workers{Mode: WorkersWrite, Agents: []string{"claude"}}
	other := Workers{Mode: WorkersRead, Agents: []string{"codex"}}
	cases := []struct {
		name     string
		w        Workers
		part     Part
		kind     string
		mode     string
		warnWord string
	}{
		{"off native", off, native, KindSpawn, "", ""},
		{"read native", rd, native, KindSpawn, "", ""},
		{"write native", wr, native, KindSpawn, "", ""},
		{"off read part", off, read, KindSpawn, "", "are off"},
		{"read read part", rd, read, KindDelegate, "read", ""},
		{"write read part", wr, read, KindDelegate, "read", ""},
		{"off edit part", off, edit, KindSpawn, "", "are off"},
		{"read edit part", rd, edit, KindSpawn, "", "needs write workers"},
		{"write edit part", wr, edit, KindDelegate, "write", ""},
		{"agent not enabled", other, read, KindSpawn, "", "not enabled (enabled: codex)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := buildNext(t, Gateway{Enabled: true, Name: "9router"}, c.w, c.part)
			s := p.Next[0]
			if s.Kind != c.kind || s.Mode != c.mode {
				t.Fatalf("step = %+v", s)
			}
			if c.kind == KindSpawn && (s.Route != "native" || s.Agent != "" || s.SubagentType == "") {
				t.Errorf("spawn step = %+v", s)
			}
			md := p.Markdown()
			if c.kind == KindSpawn && strings.Contains(md, "MCP `delegate`") {
				t.Errorf("delegate in markdown:\n%s", md)
			}
			if c.kind == KindDelegate && !strings.Contains(md, `"mode":"`+c.mode+`"`) {
				t.Errorf("markdown lacks mode %s:\n%s", c.mode, md)
			}
			joined := strings.Join(p.Warnings, "\n")
			if c.warnWord == "" && joined != "" || !strings.Contains(joined, c.warnWord) {
				t.Errorf("warnings = %q, want %q", joined, c.warnWord)
			}
			if c.warnWord != "" && !strings.Contains(joined, "DOWNSHIFT_WORKERS=") {
				t.Errorf("warning lacks the fix: %q", joined)
			}
		})
	}
}
