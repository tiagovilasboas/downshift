// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/plan"
)

const goodPlan = `{"goal":"Export","parts":[
{"id":"api","title":"Endpoint","agent":"coder","points":3,"files":["a.go"]},
{"id":"tests","title":"Tests","agent":"tester","points":2,"depends_on":["api"]}]}`

func planData(t *testing.T, d Deps, args string) planResult {
	t.Helper()
	env, err := call(t, d, "plan", args)
	if err != nil {
		t.Fatal(err)
	}
	var r planResult
	if err := json.Unmarshal(env["data"], &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestPlanFillsWavesAndRoute(t *testing.T) {
	d := fullDeps()
	d.Gateway = func() plan.Gateway { return plan.Gateway{Enabled: true, Name: "9router"} }
	r := planData(t, d, goodPlan)
	if !r.Valid || r.Plan == nil || len(r.Plan.Parts) != 2 || r.Plan.Waves != 2 {
		t.Fatalf("result = %+v", r)
	}
	if !strings.Contains(r.Markdown, "Next steps") {
		t.Fatalf("markdown = %q", r.Markdown)
	}
}

func TestPlanReturnsEveryProblem(t *testing.T) {
	r := planData(t, fullDeps(), `{"goal":"","parts":[{"id":"a","title":"","agent":"nobody","points":9}]}`)
	if r.Valid || len(r.Problems) < 2 || r.Plan != nil {
		t.Fatalf("result = %+v", r)
	}
}

func TestPlanRejectsEmptyArgsAndWorksWithoutPorts(t *testing.T) {
	mustFail(t, fullDeps(), "plan", ``, "JSON object")
	if r := planData(t, Deps{}, goodPlan); !r.Valid {
		t.Fatalf("no ports: %+v", r)
	}
}

func TestPlanRecordsCallCounts(t *testing.T) {
	type rec struct {
		tool, code string
		c          CallCounts
	}
	var got []rec
	d := fullDeps()
	d.Workers = WorkerState{Mode: "read", Agents: []string{"claude"}}
	d.OnCall = func(tool, code string, c CallCounts) { got = append(got, rec{tool, code, c}) }
	mixed := `{"goal":"g","parts":[
{"id":"a","title":"A","agent":"researcher","points":1,"worker":"claude"},
{"id":"b","title":"B","agent":"coder","points":1},
{"id":"c","title":"C","agent":"coder","points":1,"worker":"claude"}]}`
	planData(t, d, mixed)
	planData(t, d, `{"goal":"","parts":[{"id":"a","title":"","agent":"nobody","points":9}]}`)
	want := []rec{
		{"plan", "", CallCounts{Steps: 3, DelegateSteps: 1, SpawnSteps: 2}},
		{"plan", "invalid", CallCounts{}},
	}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("calls = %+v, want %+v", got, want)
	}
}
