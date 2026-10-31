// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// projectFile writes body as the project policy file of a fresh project
// directory and returns its path.
func projectFile(t *testing.T, body string) string {
	t.Helper()
	path := ProjectPath(t.TempDir())
	writeAt(t, path, body)
	return path
}

// userFile writes body as a user policy file in a fresh directory.
func userFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), FileName)
	writeAt(t, path, body)
	return path
}

func writeAt(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAbsentFilesGiveDefaults(t *testing.T) {
	for _, loc := range []Locations{
		{},
		{Project: filepath.Join(t.TempDir(), "missing.json"), User: filepath.Join(t.TempDir(), "missing.json")},
	} {
		p, err := Load(loc)
		if err != nil {
			t.Fatalf("absent files refused: %v", err)
		}
		if p.CriticalThreshold != DefaultCriticalThreshold || p.BothCritical != DefaultBothCritical || p.QuotaTTL != DefaultQuotaTTL {
			t.Errorf("defaults = %v %v %v", p.CriticalThreshold, p.BothCritical, p.QuotaTTL)
		}
		if len(p.Profiles) != 0 || len(p.Rules) != 0 {
			t.Errorf("absent files declared %d profiles, %d rules; want none", len(p.Profiles), len(p.Rules))
		}
		if _, ok := p.Match(MatchContext{Kind: contract.KindCode, Effort: contract.EffortDeep}); ok {
			t.Error("empty policy matched a rule; no rule must mean the requested model is kept")
		}
	}
}

// A file the tier floor owns, with no routing key, is not a routing error.
func TestFileWithoutRoutingIsNotAnError(t *testing.T) {
	path := projectFile(t, `{"version":1,"min_tier":"mid","min_tier_by_class":{"COMPLEX":"frontier"}}`)
	p, err := Load(Locations{Project: path})
	if err != nil {
		t.Fatalf("tier-floor file refused: %v", err)
	}
	if len(p.Rules) != 0 || p.CriticalThreshold != DefaultCriticalThreshold {
		t.Fatalf("tier-floor file produced routing settings: %+v", p)
	}
}

func TestValidFileLoadsEverySetting(t *testing.T) {
	path := projectFile(t, `{"version":1,"routing":{
	  "critical_threshold":0,"both_critical":"run-cheapest","quota_ttl_seconds":120,
	  "profiles":[
	    {"name":"reviewer","tier":"frontier","harness":"claude-code","fallback":"careful","description":"Reviews a diff"},
	    {"name":"careful","tier":"mid","harness":"claude-code"}],
	  "rules":[
	    {"id":"docs","kinds":["docs"],"efforts":["light"],"profile":"careful",
	     "gates":[{"harness":"claude-code","window":"week","below":80.5}]},
	    {"id":"fallback-tier","enabled":false,"tier":"small"}]
	}}`)
	p, err := Load(Locations{Project: path})
	if err != nil {
		t.Fatal(err)
	}
	if p.CriticalThreshold != 0 || p.BothCritical != BothCriticalRunCheapest || p.QuotaTTL != 2*time.Minute {
		t.Errorf("settings = %v %v %v", p.CriticalThreshold, p.BothCritical, p.QuotaTTL)
	}
	if len(p.Profiles) != 2 || p.Profiles[0].Name != "reviewer" || p.Profiles[1].Fallback != "" {
		t.Errorf("profiles = %+v", p.Profiles)
	}
	if p.Profiles[0].Description != "Reviews a diff" || p.Profiles[1].Description != "" {
		t.Errorf("descriptions = %q, %q", p.Profiles[0].Description, p.Profiles[1].Description)
	}
	docs := p.Rules[0]
	if docs.ID != "docs" || !docs.Enabled || docs.Profile != "careful" || docs.Tier != contract.TierUnknown {
		t.Errorf("rule docs = %+v", docs)
	}
	if want := (Gate{Harness: "claude-code", Window: contract.WindowWeek, Below: 80.5}); !reflect.DeepEqual(docs.Gates, []Gate{want}) {
		t.Errorf("gates = %+v, want %+v", docs.Gates, want)
	}
	if p.Rules[1].Enabled || p.Rules[1].Tier != contract.TierSmall {
		t.Errorf("disabled rule = %+v", p.Rules[1])
	}
}

// The project file wins over the user file for every setting it declares, and
// its profiles and rules replace the user's with the same name or id. Project
// rules come first, so a project rule is tried before a user rule.
func TestProjectWinsOverUser(t *testing.T) {
	user := userFile(t, `{"version":1,"routing":{
	  "critical_threshold":70,"quota_ttl_seconds":60,"both_critical":"block",
	  "profiles":[
	    {"name":"reviewer","tier":"small","harness":"claude-code"},
	    {"name":"docs","tier":"small","harness":"claude-code"}],
	  "rules":[
	    {"id":"shared","tier":"small"},
	    {"id":"user-only","tier":"mid"}]
	}}`)
	project := projectFile(t, `{"version":1,"routing":{
	  "critical_threshold":90,
	  "profiles":[{"name":"reviewer","tier":"frontier","harness":"claude-code"}],
	  "rules":[
	    {"id":"project-first","kinds":["review"],"profile":"reviewer"},
	    {"id":"shared","kinds":["code"],"tier":"frontier"}]
	}}`)
	p, err := Load(Locations{Project: project, User: user})
	if err != nil {
		t.Fatal(err)
	}
	if p.CriticalThreshold != 90 {
		t.Errorf("threshold = %v, want the project's 90", p.CriticalThreshold)
	}
	if p.QuotaTTL != time.Minute || p.BothCritical != BothCriticalBlock {
		t.Errorf("unset in project, so user's values: ttl=%v both=%v", p.QuotaTTL, p.BothCritical)
	}
	gotProfiles := []contract.ProfileName{p.Profiles[0].Name, p.Profiles[1].Name}
	if !reflect.DeepEqual(gotProfiles, []contract.ProfileName{"reviewer", "docs"}) || p.Profiles[0].Tier != contract.TierFrontier {
		t.Errorf("profiles = %+v", p.Profiles)
	}
	var ids []contract.RuleID
	for _, r := range p.Rules {
		ids = append(ids, r.ID)
	}
	if !reflect.DeepEqual(ids, []contract.RuleID{"project-first", "shared", "user-only"}) {
		t.Errorf("rule order = %v", ids)
	}
	if p.Rules[1].Kinds[0] != contract.KindCode || p.Rules[1].Tier != contract.TierFrontier {
		t.Errorf("shared rule kept the user's version: %+v", p.Rules[1])
	}
	if p.Rules[0].source != project || p.Rules[2].source != user {
		t.Errorf("sources = %q, %q", p.Rules[0].source, p.Rules[2].source)
	}
}

// A project search can reach the user's own directory. The same file must not
// be read as two layers, which would duplicate every rule.
func TestSameFileIsReadOnce(t *testing.T) {
	path := projectFile(t, `{"version":1,"routing":{"rules":[{"id":"only","tier":"mid"}]}}`)
	p, err := Load(Locations{Project: path, User: path})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(p.Rules))
	}
}

func TestBrokenFileRefusesWholeLoad(t *testing.T) {
	project := projectFile(t, `{"version":1,"routing":{"critical_threshold":80}}`)
	user := userFile(t, `{"version":1,"routing":{"critical_threshold":150}}`)
	p, err := Load(Locations{Project: project, User: user})
	if err == nil || p != nil {
		t.Fatalf("Load = %v, %v; a broken user file must refuse the whole load", p, err)
	}
}

// The same files always give the same policy and the same error text.
func TestLoadIsDeterministic(t *testing.T) {
	user := userFile(t, `{"version":1,"routing":{"profiles":[{"name":"a","tier":"mid","harness":"codex"}],"rules":[{"id":"r1","kinds":["code"],"tier":"mid"},{"id":"r2","tier":"small"}]}}`)
	first, err1 := Load(Locations{User: user})
	second, err2 := Load(Locations{User: user})
	if err1 != nil || err2 != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("two loads differ: %+v (%v) vs %+v (%v)", first, err1, second, err2)
	}
	bad := userFile(t, `{"version":1,"routing":{"rules":[{"id":"x","tier":"mid"},{"id":"x","tier":"mid"}]}}`)
	_, e1 := Load(Locations{User: bad})
	_, e2 := Load(Locations{User: bad})
	if e1 == nil || e1.Error() != e2.Error() {
		t.Fatalf("errors differ: %v vs %v", e1, e2)
	}
}

func TestUserPathFollowsStateDir(t *testing.T) {
	state := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", state)
	got, err := UserPath()
	if err != nil || got != filepath.Join(state, FileName) {
		t.Fatalf("UserPath = %q, %v; want the state directory's %s", got, err, FileName)
	}
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("DOWNSHIFT_STATE_DIR", "")
	got, err = UserPath()
	if err != nil || got != filepath.Join(home, ".downshift", FileName) {
		t.Fatalf("default UserPath = %q, %v; want under the isolated home", got, err)
	}
}
