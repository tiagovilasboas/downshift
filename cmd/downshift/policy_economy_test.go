// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/policyfile"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// economyRun runs 'downshift policy economy' with stdin and returns the exit
// code and the two output streams.
func economyRun(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errw bytes.Buffer
	rc := runPolicyEconomy(args, strings.NewReader(stdin), &out, &errw)
	return rc, out.String(), errw.String()
}

// readPolicy returns the project file's text, failing the test when it is absent.
func readPolicy(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEconomyOnCreatesProjectFile(t *testing.T) {
	project := isolatePolicy(t)
	path := policy.ProjectPath(project)
	if rc, _, errw := economyRun(t, "", "on", "--yes"); rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(readPolicy(t, path)), &doc); err != nil {
		t.Fatalf("written file is not JSON: %v", err)
	}
	if doc["version"] != float64(1) || doc["mode"] != "economy" || len(doc) != 2 {
		t.Fatalf("new file = %v, want version 1 and mode economy only", doc)
	}
	p, err := policy.Load(policy.Locations{Project: path})
	if err != nil || !p.EconomyMode() {
		t.Fatalf("written file does not load as economy: %v", err)
	}
}

// TestEconomyToggleKeepsOtherSettings checks that the rewrite keeps the tier
// floor and the routing section, and that off writes standard.
func TestEconomyToggleKeepsOtherSettings(t *testing.T) {
	project := isolatePolicy(t)
	path := policy.ProjectPath(project)
	writeProjectPolicy(t, project, `{"version":1,"min_tier":"mid","routing":{"critical_threshold":70,"rules":[{"id":"review","kinds":["review"],"tier":"frontier"}]}}`)
	if rc, _, errw := economyRun(t, "", "on", "--yes"); rc != 0 {
		t.Fatalf("on: rc = %d, stderr %q", rc, errw)
	}
	after := readPolicy(t, path)
	for _, want := range []string{`"min_tier": "mid"`, `"critical_threshold": 70`, `"mode": "economy"`, `"id": "review"`} {
		if !strings.Contains(after, want) {
			t.Fatalf("after on, file lacks %s:\n%s", want, after)
		}
	}
	if _, err := policyfile.Load(path); err != nil {
		t.Fatalf("floor refused the rewritten file: %v", err)
	}
	if p, err := policy.Load(policy.Locations{Project: path}); err != nil || !p.EconomyMode() || len(p.Rules) != 1 {
		t.Fatalf("routing after on: err %v, economy %v", err, p != nil && p.EconomyMode())
	}
	if rc, _, errw := economyRun(t, "", "off", "--yes"); rc != 0 {
		t.Fatalf("off: rc = %d, stderr %q", rc, errw)
	}
	if !strings.Contains(readPolicy(t, path), `"mode": "standard"`) {
		t.Fatalf("after off, mode is not standard:\n%s", readPolicy(t, path))
	}
}

// TestEconomyOffInProjectOverridesUserOn checks that a project 'off' wins over
// economy turned on in the user's file.
func TestEconomyOffInProjectOverridesUserOn(t *testing.T) {
	project := isolatePolicy(t)
	if rc, _, errw := economyRun(t, "", "on", "--user", "--yes"); rc != 0 {
		t.Fatalf("user on: rc = %d, stderr %q", rc, errw)
	}
	userPath, err := policy.UserPath()
	if err != nil {
		t.Fatal(err)
	}
	if rc, _, errw := economyRun(t, "", "off", "--yes"); rc != 0 {
		t.Fatalf("project off: rc = %d, stderr %q", rc, errw)
	}
	projectPath := policy.ProjectPath(project)
	p, err := policy.Load(policy.Locations{Project: projectPath, User: userPath})
	if err != nil {
		t.Fatal(err)
	}
	if p.EconomyMode() {
		t.Fatal("project off did not override the user's economy")
	}
}

func TestEconomyDryRunWritesNothing(t *testing.T) {
	project := isolatePolicy(t)
	rc, out, errw := economyRun(t, "", "on", "--dry-run")
	if rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw)
	}
	if !strings.Contains(out, "would change") || !strings.Contains(out, "Dry run") {
		t.Fatalf("dry run output = %q", out)
	}
	if _, err := os.Stat(policy.ProjectPath(project)); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote the file: %v", err)
	}
}

func TestEconomyWithoutYesAsksAndKeepsFile(t *testing.T) {
	project := isolatePolicy(t)
	path := policy.ProjectPath(project)
	writeProjectPolicy(t, project, `{"version":1}`)
	before := readPolicy(t, path)
	rc, out, _ := economyRun(t, "n\n", "on")
	if rc != 1 {
		t.Fatalf("rc = %d, want 1 when the answer is no", rc)
	}
	if !strings.Contains(out, "Write these changes?") || !strings.Contains(out, "Not applied") {
		t.Fatalf("out = %q", out)
	}
	if readPolicy(t, path) != before {
		t.Fatal("a declined change still wrote the file")
	}
}

// TestEconomyRefusesUnusableFile checks that the toggle does not rewrite a file
// the loaders would refuse: the file stays as it was and the command fails.
func TestEconomyRefusesUnusableFile(t *testing.T) {
	project := isolatePolicy(t)
	path := policy.ProjectPath(project)
	writeProjectPolicy(t, project, `{"version":1,"routing":{"critical_threshold":150}}`)
	before := readPolicy(t, path)
	rc, _, errw := economyRun(t, "", "on", "--yes")
	if rc != 1 || !strings.Contains(errw, "routing.critical_threshold") {
		t.Fatalf("rc = %d, stderr %q; want 1 with the field named", rc, errw)
	}
	if readPolicy(t, path) != before {
		t.Fatal("an unusable file was rewritten")
	}
}

func TestEconomyUserFlagWritesStateDirectory(t *testing.T) {
	isolatePolicy(t)
	if rc, _, errw := economyRun(t, "", "on", "--user", "--yes"); rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw)
	}
	userPath, err := policy.UserPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(userPath) != policy.FileName {
		t.Fatalf("user path %q", userPath)
	}
	if p, err := policy.Load(policy.Locations{User: userPath}); err != nil || !p.EconomyMode() {
		t.Fatalf("user file not economy: %v", err)
	}
}

func TestEconomyArgumentErrors(t *testing.T) {
	isolatePolicy(t)
	cases := [][]string{
		nil,
		{"maybe"},
		{"on", "off"},
		{"on", "--bogus"},
	}
	for _, args := range cases {
		if rc, _, _ := economyRun(t, "", args...); rc != 2 {
			t.Errorf("args %v: rc = %d, want 2", args, rc)
		}
	}
}

func TestPolicyDispatchesEconomy(t *testing.T) {
	isolatePolicy(t)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"economy", "on", "--dry-run"}, &out, &errw); rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw.String())
	}
	if !strings.Contains(out.String(), "would change") {
		t.Fatalf("out = %q", out.String())
	}
}
