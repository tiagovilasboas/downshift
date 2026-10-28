// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
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
			p, ok := planPolicyInit(setupOptions{Home: home, ProjectDir: tt.projectDir})
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if p.changed() != tt.wantChanged {
				t.Errorf("changed = %v, want %v (skip %q)", p.changed(), tt.wantChanged, p.skip)
			}
			if (p.skip != "") != tt.wantSkip {
				t.Errorf("skip = %q, want skip %v", p.skip, tt.wantSkip)
			}
			if string(p.after) != policyInitDocument {
				t.Errorf("after = %q", p.after)
			}
		})
	}
}

func TestPolicyInitCommand(t *testing.T) {
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
		{name: "yes writes the file", args: []string{"--yes"}, wantRC: 0, wantFile: true, wantBody: policyInitDocument},
		{name: "answering y writes the file", input: "y\n", wantRC: 0, wantFile: true, wantBody: policyInitDocument},
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
	if !strings.Contains(out.String(), "policy ok: 0 profiles, 0 rules") {
		t.Errorf("new file is not an empty routing policy: %q", out.String())
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
			opts := setupOptions{
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

	opts := setupOptions{
		Home:       home,
		ProjectDir: projDir,
		Yes:        true,
		In:         strings.NewReader(""),
		Out:        io.Discard,
		Err:        io.Discard,
		Now:        time.Now,
	}

	plan, ok := planPolicyInit(opts)
	if !ok {
		t.Fatal("planPolicyInit not ok")
	}
	if failed, _ := applyPlans(opts, []setupPlan{plan}); failed {
		t.Fatal("applyPlans failed")
	}

	policyPath := policy.ProjectPath(projDir)
	if _, err := os.Stat(policyPath); err != nil {
		t.Fatalf("expected policy.json to exist: %v", err)
	}

	// Now undo
	opts.Undo = true
	var outBuf bytes.Buffer
	opts.Out = &outBuf
	if code := setupUndo(opts, setupTargets); code != 0 {
		t.Fatalf("setupUndo exit %d\n%s", code, outBuf.String())
	}

	if _, err := os.Stat(policyPath); !os.IsNotExist(err) {
		t.Fatalf("expected policy.json to be removed by undo: %v", err)
	}
}
