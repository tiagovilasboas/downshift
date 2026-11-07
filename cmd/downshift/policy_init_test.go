// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

func TestPlanPolicyInit(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	existing := policy.ProjectPath(project)
	if err := os.MkdirAll(filepath.Dir(existing), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(existing, []byte(`{"version":1}`), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh := t.TempDir()

	tests := []struct {
		name        string
		projectDir  string
		wantOK      bool
		wantChanged bool
		wantSkip    bool
	}{
		{name: "no project directory skips the step", projectDir: "", wantOK: false},
		{name: "absent file is planned", projectDir: fresh, wantOK: true, wantChanged: true},
		{name: "existing file is left untouched", projectDir: project, wantOK: true, wantSkip: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, ok := setup.PlanPolicyInit(setup.Options{Home: home, ProjectDir: tt.projectDir})
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if p.Changed() != tt.wantChanged {
				t.Errorf("changed = %v, want %v (skip %q)", p.Changed(), tt.wantChanged, p.Skip)
			}
			if (p.Skip != "") != tt.wantSkip {
				t.Errorf("skip = %q, want skip %v", p.Skip, tt.wantSkip)
			}
			if string(p.After) != setup.PolicyInitDocument {
				t.Errorf("after = %q", p.After)
			}
		})
	}
}

func TestPolicyInitCommand(t *testing.T) {
	// In a clean home only Claude Code resolves a small model (the catalog
	// stand-in), so the default file carries that one profile.
	exploreBody, _ := setup.PolicyInitDocumentFor([]setup.SmallTarget{{Harness: "claude-code", Model: "x", Source: "catalog-fallback"}}, false)
	tests := []struct {
		name       string
		args       []string
		input      string
		existing   string
		wantRC     int
		wantFile   bool
		wantOutHas string
		wantBody   string // the file content afterwards, when it exists
	}{
		{name: "dry run writes nothing", args: []string{"--dry-run"}, wantRC: 0, wantOutHas: "Dry run: nothing was written."},
		{name: "declined question writes nothing", input: "n\n", wantRC: 1},
		{name: "yes writes the file", args: []string{"--yes"}, wantRC: 0, wantFile: true, wantBody: exploreBody, wantOutHas: "profile \"explore\""},
		{name: "answering y writes the file", input: "y\n", wantRC: 0, wantFile: true, wantBody: exploreBody},
		{name: "bare writes only the version", args: []string{"--yes", "--bare"}, wantRC: 0, wantFile: true, wantBody: setup.PolicyInitDocument},
		{name: "existing file is left untouched", args: []string{"--yes"}, existing: `{"version":1,"min_tier":"mid"}`, wantRC: 0, wantFile: true, wantBody: `{"version":1,"min_tier":"mid"}`, wantOutHas: "left untouched"},
		{name: "unknown argument is a usage error", args: []string{"--bogus"}, wantRC: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project := isolatePolicy(t)
			path := policy.ProjectPath(project)
			if tt.existing != "" {
				writeProjectPolicy(t, project, tt.existing)
			}
			var out, errw bytes.Buffer
			rc := runPolicyInit(tt.args, strings.NewReader(tt.input), &out, &errw)
			if rc != tt.wantRC {
				t.Fatalf("rc = %d, want %d; stderr %q", rc, tt.wantRC, errw.String())
			}
			if tt.wantOutHas != "" && !strings.Contains(out.String(), tt.wantOutHas) {
				t.Errorf("out = %q, want it to contain %q", out.String(), tt.wantOutHas)
			}
			body, err := os.ReadFile(path)
			if !tt.wantFile {
				if err == nil {
					t.Fatalf("file written: %q", body)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestPolicyInitFileIsAValidPolicy(t *testing.T) {
	project := isolatePolicy(t)
	var out, errw bytes.Buffer
	if rc := runPolicyInit([]string{"--yes"}, strings.NewReader(""), &out, &errw); rc != 0 {
		t.Fatalf("init rc = %d, stderr %q", rc, errw.String())
	}
	out.Reset()
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 0 {
		t.Fatalf("policy check rc = %d, stderr %q", rc, errw.String())
	}
	if !strings.Contains(out.String(), "project file: "+policy.ProjectPath(project)) {
		t.Errorf("check did not find the new file: %q", out.String())
	}
	if !strings.Contains(out.String(), "policy ok: 1 profiles, 0 rules") {
		t.Errorf("new file does not carry the one explore profile: %q", out.String())
	}
}

// The planner writes one explore profile per harness with a small model, says
// what each resolves to, and leaves out (with a note) a harness with no list.
func TestPlanPolicyInitExploreProfile(t *testing.T) {
	resolved := setup.SmallTarget{Harness: "claude-code", Model: "claude-haiku-4-5", Native: "haiku", Source: "catalog-fallback"}
	unresolved := setup.SmallTarget{Harness: "cursor"}
	cases := []struct {
		name         string
		targets      []setup.SmallTarget
		bare         bool
		wantProfiles int
		wantNotes    []string
	}{
		{"no targets", nil, false, 0, nil},
		{"one resolved, one not", []setup.SmallTarget{unresolved, resolved}, false, 1,
			[]string{"claude-code: profile \"explore\"", `claude-haiku-4-5 (written as "haiku")`, "catalog-fallback", "cursor: no detected model list", "downshift models discover --harness cursor"}},
		{"bare keeps the version only", []setup.SmallTarget{resolved}, true, 0, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ok := setup.PlanPolicyInit(setup.Options{Home: t.TempDir(), ProjectDir: t.TempDir(), SmallTargets: c.targets, BarePolicy: c.bare})
			if !ok || p.Err != nil || p.Skip != "" {
				t.Fatalf("plan ok=%v err=%v skip=%q", ok, p.Err, p.Skip)
			}
			var doc struct {
				Version int `json:"version"`
				Routing *struct {
					Profiles []struct{ Name, Tier, Harness, Description string } `json:"profiles"`
				} `json:"routing"`
			}
			if err := json.Unmarshal(p.After, &doc); err != nil || doc.Version != 1 {
				t.Fatalf("file is not a version-1 policy: %v\n%s", err, p.After)
			}
			got := 0
			if doc.Routing != nil {
				got = len(doc.Routing.Profiles)
				for _, pr := range doc.Routing.Profiles {
					if pr.Name != setup.ExploreProfile || pr.Tier != "small" || pr.Harness != "claude-code" || pr.Description == "" {
						t.Errorf("profile = %+v", pr)
					}
				}
			}
			if got != c.wantProfiles {
				t.Fatalf("%d profile(s), want %d:\n%s", got, c.wantProfiles, p.After)
			}
			if c.wantProfiles == 0 && string(p.After) != setup.PolicyInitDocument {
				t.Errorf("no profile should give the bare document, got %s", p.After)
			}
			notes := strings.Join(p.Notes, "\n")
			for _, w := range c.wantNotes {
				if !strings.Contains(notes, w) {
					t.Errorf("notes lack %q:\n%s", w, notes)
				}
			}
		})
	}
}

func TestSetupOffersPolicyOnlyWithAProjectDirectory(t *testing.T) {
	tests := []struct {
		name       string
		projectSet bool
		dryRun     bool
		input      string
		wantFile   bool
		wantOutHas string
	}{
		{name: "no project directory: no policy step", projectSet: false, wantFile: false},
		{name: "dry run shows the file and writes nothing", projectSet: true, dryRun: true, wantFile: false, wantOutHas: "policy: "},
		{name: "declined question writes no policy", projectSet: true, input: "n\n", wantFile: false},
		{name: "yes writes the policy", projectSet: true, input: "y\n", wantFile: true, wantOutHas: "policy: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newSetupEnv(t, ".claude")
			project := t.TempDir()
			var out, errw bytes.Buffer
			opts := setup.Options{
				Home: env.home, Binary: testBin, SkipDiscovery: true,
				Names: []string{"claude-code"}, DryRun: tt.dryRun,
				In:  strings.NewReader(tt.input),
				Out: &out, Err: &errw,
				Now: func() time.Time { return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC) },
			}
			if tt.projectSet {
				opts.ProjectDir = project
			}
			setupRun(opts)
			_, err := os.Stat(policy.ProjectPath(project))
			if got := err == nil; got != tt.wantFile {
				t.Fatalf("policy file present = %v, want %v; out %q", got, tt.wantFile, out.String())
			}
			if tt.wantOutHas != "" && !strings.Contains(out.String(), tt.wantOutHas) {
				t.Errorf("out = %q, want it to contain %q", out.String(), tt.wantOutHas)
			}
		})
	}
}

func TestSetupUndoRestoresPolicyInit(t *testing.T) {
	home := t.TempDir()
	projDir := t.TempDir()

	opts := setup.Options{
		Home:       home,
		ProjectDir: projDir,
		Yes:        true,
		In:         strings.NewReader(""),
		Out:        io.Discard,
		Err:        io.Discard,
		Now:        time.Now,
	}

	plan, ok := setup.PlanPolicyInit(opts)
	if !ok {
		t.Fatal("setup.PlanPolicyInit not ok")
	}
	if failed, _ := setup.ApplyPlans(opts, []setup.Plan{plan}); failed {
		t.Fatal("setup.ApplyPlans failed")
	}

	policyPath := policy.ProjectPath(projDir)
	if _, err := os.Stat(policyPath); err != nil {
		t.Fatalf("expected policy.json to exist: %v", err)
	}

	// Now undo
	opts.Undo = true
	var outBuf bytes.Buffer
	opts.Out = &outBuf
	if code := setup.Undo(opts, setup.Targets); code != 0 {
		t.Fatalf("setup.Undo exit %d\n%s", code, outBuf.String())
	}

	if _, err := os.Stat(policyPath); !os.IsNotExist(err) {
		t.Fatalf("expected policy.json to be removed by undo: %v", err)
	}
}
