// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/playbook"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/agenttier"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/contract"
)

var shippedIDs = playbook.ShippedIDs()

func part(id, agent string, pts int, deps ...string) Part {
	return Part{ID: id, Title: "t " + id, Agent: agent, Points: pts, DependsOn: deps}
}

func TestValidation(t *testing.T) {
	cases := []struct {
		name  string
		parts []Part
		want  []string
	}{
		{"ok", []Part{part("a", "coder", 3)}, nil},
		{"empty", nil, []string{"no parts"}},
		{"empty id", []Part{part("", "coder", 3)}, []string{"id is empty"}},
		{"duplicate", []Part{part("a", "coder", 1), part("a", "coder", 1)}, []string{"duplicate id"}},
		{"bad agent", []Part{part("a", "wizard", 1)}, []string{"unknown agent"}},
		{"eight", []Part{part("a", "coder", 8)}, []string{"split this part"}},
		{"thirteen", []Part{part("a", "coder", 13)}, []string{"split this part"}},
		{"four", []Part{part("a", "coder", 4)}, []string{"not one of"}},
		{"zero", []Part{part("a", "coder", 0)}, []string{"not one of"}},
		{"missing dep", []Part{part("a", "coder", 1, "z")}, []string{`"z" does not exist`}},
		{"cycle", []Part{part("a", "coder", 1, "b"), part("b", "coder", 1, "a")}, []string{"cycle"}},
		{"self cycle", []Part{part("a", "coder", 1, "a")}, []string{"cycle"}},
		{"bad worker", []Part{{ID: "a", Agent: "coder", Points: 1, Worker: "gpt"}}, []string{"unknown worker"}},
		{"bad playbook", []Part{{ID: "a", Agent: "coder", Points: 1, Playbook: "nope"}}, []string{"playbook"}},
		{"all errors", []Part{part("a", "wizard", 8), part("b", "coder", 4)}, []string{"unknown agent", "split this part", "not one of"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, errs := Build(Input{Parts: c.parts}, Gateway{}, Workers{}, shippedIDs)
			joined := strings.Join(errs, "\n")
			if len(c.want) == 0 && len(errs) > 0 {
				t.Fatalf("unexpected errors: %s", joined)
			}
			for _, w := range c.want {
				if !strings.Contains(joined, w) {
					t.Errorf("errors %q lack %q", joined, w)
				}
			}
		})
	}
}

func TestFillAndWaves(t *testing.T) {
	in := Input{Goal: "g", Parts: []Part{
		part("c", "reviewer", 2, "a", "b"),
		part("a", "coder", 3),
		part("b", "Architect", 2, "a"),
		{ID: "d", Agent: "coder", Points: 1, Playbook: "Docs"},
	}}
	p, errs := Build(in, Gateway{}, Workers{}, shippedIDs)
	if errs != nil {
		t.Fatal(errs)
	}
	got := map[string]FilledPart{}
	for _, q := range p.Parts {
		got[q.ID] = q
	}
	for id, wave := range map[string]int{"a": 1, "b": 2, "c": 3, "d": 1} {
		if got[id].Wave != wave {
			t.Errorf("%s wave = %d, want %d", id, got[id].Wave, wave)
		}
	}
	if got["a"].Playbook != "implementation" || got["c"].Playbook != "review" || got["d"].Playbook != "docs" {
		t.Errorf("playbooks: %+v", got)
	}
	if got["b"].Tier != string(contract.TierFrontier) || got["b"].Agent != "architect" {
		t.Errorf("b = %+v", got["b"])
	}
	if p.Points != 8 || p.Waves != 3 {
		t.Errorf("points=%d waves=%d", p.Points, p.Waves)
	}
}

func TestRoutes(t *testing.T) {
	cases := []struct {
		worker string
		gw     Gateway
		want   string
	}{
		{"", Gateway{}, "native"},
		{"native", Gateway{Enabled: true, Name: "9router"}, "native"},
		{"claude", Gateway{}, "direct"},
		{"codex", Gateway{Enabled: false, Name: "9router"}, "direct"},
		{"claude", Gateway{Enabled: true, Name: "9router"}, "9router"},
		{"codex", Gateway{Enabled: true, Name: "9router"}, "9router"},
	}
	for _, c := range cases {
		in := Input{Parts: []Part{{ID: "a", Agent: "coder", Points: 1, Worker: c.worker}}}
		p, errs := Build(in, c.gw, Workers{Mode: WorkersWrite, Agents: []string{"claude", "codex"}}, shippedIDs)
		if errs != nil {
			t.Fatal(errs)
		}
		if p.Parts[0].Route != c.want {
			t.Errorf("%+v: route %q, want %q", c, p.Parts[0].Route, c.want)
		}
	}
}

func TestSharedFileWarning(t *testing.T) {
	a, b, c := part("a", "coder", 1), part("b", "coder", 1), part("c", "coder", 1, "a")
	a.Files, b.Files, c.Files = []string{"x.go"}, []string{"x.go"}, []string{"x.go"}
	p, errs := Build(Input{Parts: []Part{a, b, c}}, Gateway{}, Workers{}, shippedIDs)
	if errs != nil {
		t.Fatal(errs)
	}
	if len(p.Warnings) != 1 || !strings.Contains(p.Warnings[0], "a, b share file x.go") {
		t.Errorf("warnings = %v", p.Warnings)
	}
}

func TestRender(t *testing.T) {
	a := part("a", "coder", 3)
	b := part("b", "tester", 2, "a")
	b.Title = "pipe | title"
	p, _ := Build(Input{Goal: "ship it", Parts: []Part{a, b}}, Gateway{}, Workers{}, shippedIDs)
	md := p.Markdown()
	for _, w := range []string{
		"## Plan: ship it",
		"| # | Part | Agent | Tier | Playbook | Pts | Depends on | Wave | Route |",
		"| a | t a | coder | mid | implementation | 3 | - | 1 | native |",
		`pipe \| title`,
		"Total: 5 points in 2 waves.",
	} {
		if !strings.Contains(md, w) {
			t.Errorf("markdown lacks %q:\n%s", w, md)
		}
	}
	js, err := p.JSON()
	if err != nil || !strings.Contains(string(js), `"total_points": 5`) {
		t.Errorf("json = %s, %v", js, err)
	}
}

func TestEveryShippedAgentNamesAnExistingPlaybook(t *testing.T) {
	ids := map[string]bool{}
	for _, id := range shippedIDs {
		ids[id] = true
	}
	for _, e := range agenttier.List() {
		if e.Playbook == "" || !ids[e.Playbook] {
			t.Errorf("agent %s names playbook %q, which does not exist", e.Agent, e.Playbook)
		}
	}
}

func TestParseInput(t *testing.T) {
	if _, err := ParseInput([]byte(`{bad`)); err == nil {
		t.Error("want error")
	}
	in, err := ParseInput([]byte(`{"goal":"g","parts":[{"id":"a","agent":"coder","points":2}]}`))
	if err != nil || len(in.Parts) != 1 {
		t.Errorf("%+v %v", in, err)
	}
}
