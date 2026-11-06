// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

type fakeReporter struct {
	out ReportOutcome
	err error
	got *[]string
}

func (f fakeReporter) Report(_ context.Context, _ string, ids []string) (ReportOutcome, error) {
	if f.got != nil {
		*f.got = ids
	}
	return f.out, f.err
}

func reportedOf(t *testing.T, env map[string]json.RawMessage) map[string]any {
	t.Helper()
	var data struct {
		Reported map[string]any `json:"reported"`
	}
	if err := json.Unmarshal(env["data"], &data); err != nil {
		t.Fatal(err)
	}
	return data.Reported
}

func TestProvidersReportsModels(t *testing.T) {
	var got []string
	d := fullDeps()
	d.Reporter = fakeReporter{out: ReportOutcome{Accepted: 2, Written: true, Reason: "ok"}, got: &got}
	env, err := call(t, d, "providers", `{"harness":"codex","models":[" m1 ","","m2","  "]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("ids = %q", got)
	}
	r := reportedOf(t, env)
	if r["accepted"] != float64(2) || r["written"] != true || r["reason"] != "ok" {
		t.Fatalf("reported = %v", r)
	}
	if !strings.Contains(string(env["data"]), `"models":[`) || len(env["strain"]) == 0 {
		t.Fatalf("list or strain missing: %s", env["data"])
	}
}

func TestProvidersReportUnavailable(t *testing.T) {
	d := fullDeps()
	d.Reporter = fakeReporter{err: errPort}
	for name, dep := range map[string]Deps{"error": d, "nil": fullDeps()} {
		env, err := call(t, dep, "providers", `{"harness":"codex","models":["m1"]}`)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if r := reportedOf(t, env); r["unavailable"] != true || len(r) != 1 {
			t.Fatalf("%s: reported = %v", name, r)
		}
		if strings.Contains(string(env["data"]), "/secret") {
			t.Fatalf("%s: leaked port error", name)
		}
	}
}

func TestProvidersEmptyModelsBehavesAsAbsent(t *testing.T) {
	var got []string
	d := fullDeps()
	d.Reporter = fakeReporter{got: &got}
	for _, args := range []string{`{"harness":"codex","models":[]}`, `{"harness":"codex","models":null}`, `{"harness":"codex","models":["",""]}`} {
		env, err := call(t, d, "providers", args)
		if err != nil || got != nil || reportedOf(t, env) != nil {
			t.Fatalf("%s: err %v got %v", args, err, got)
		}
	}
}

func TestProvidersModelsValidation(t *testing.T) {
	d := fullDeps()
	d.Reporter = fakeReporter{}
	many, _ := json.Marshal(map[string]any{"harness": "codex", "models": make([]string, 65)})
	long, _ := json.Marshal(map[string]any{"harness": "codex", "models": []string{strings.Repeat("L", 129)}})
	mustFail(t, d, "providers", `{"harness":"codex","models":"m1"}`, "array of strings")
	mustFail(t, d, "providers", `{"harness":"codex","models":[1,2]}`, "array of strings")
	mustFail(t, d, "providers", `{"harness":"codex","models":["secretvalue",{"a":1}]}`, "array of strings")
	mustFail(t, d, "providers", string(many), "more than 64")
	mustFail(t, d, "providers", string(long), "longer than 128")
	mustFail(t, d, "providers", `{"harness":"codex","models":["m"],"x":1}`, "unknown field", "harness, models")
	if _, err := call(t, d, "providers", `{"harness":"codex","models":[`+strings.Repeat(`"m",`, 63)+`"m"]}`); err != nil {
		t.Fatalf("64 items accepted: %v", err)
	}
}

func TestProvidersSchemaAndNames(t *testing.T) {
	ts := Info(fullDeps())
	if err := Validate(ts); err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`^[a-z0-9_]+$`)
	for _, tl := range ts {
		if !re.MatchString(tl.Name) {
			t.Errorf("name %q", tl.Name)
		}
	}
	var s struct {
		Props map[string]struct {
			MaxItems int `json:"maxItems"`
		} `json:"properties"`
		Additional bool `json:"additionalProperties"`
	}
	if err := json.Unmarshal(toolByName(t, fullDeps(), "providers").Schema(), &s); err != nil || s.Props["models"].MaxItems != 64 || s.Additional {
		t.Fatalf("schema: %v %+v", err, s)
	}
}
