// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/routing/policy"
	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// isolatePolicy points HOME and the state directory at fresh temp dirs and
// makes the temp dir the working directory, so no real file is read.
func isolatePolicy(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(home, ".downshift"))
	project := t.TempDir()
	t.Chdir(project)
	return project
}

func writeProjectPolicy(t *testing.T, project, body string) {
	t.Helper()
	path := policy.ProjectPath(project)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyCheckAcceptsAbsentFiles(t *testing.T) {
	isolatePolicy(t)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw.String())
	}
	if !strings.Contains(out.String(), "policy ok: 0 profiles, 0 rules") || !strings.Contains(out.String(), "project file: none") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestPolicyCheckAcceptsFloorAndRoutingInOneFile(t *testing.T) {
	project := isolatePolicy(t)
	writeProjectPolicy(t, project, `{"version":1,"min_tier":"mid","routing":{"profiles":[{"name":"reviewer","tier":"frontier","harness":"claude-code"}],"rules":[{"id":"review","kinds":["review"],"profile":"reviewer"}]}}`)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 0 {
		t.Fatalf("rc = %d, stderr %q", rc, errw.String())
	}
	if !strings.Contains(out.String(), "policy ok: 1 profiles, 1 rules") {
		t.Fatalf("out = %q", out.String())
	}
}

func TestPolicyCheckRefusesWithTheField(t *testing.T) {
	project := isolatePolicy(t)
	writeProjectPolicy(t, project, `{"version":1,"routing":{"critical_threshold":150}}`)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if !strings.Contains(errw.String(), "routing.critical_threshold") {
		t.Fatalf("stderr = %q, want the field named", errw.String())
	}
}

func TestPolicyCheckReportsFloorRefusal(t *testing.T) {
	project := isolatePolicy(t)
	writeProjectPolicy(t, project, `{"version":1,"min_tier":"huge","routing":{}}`)
	var out, errw bytes.Buffer
	if rc := runPolicy([]string{"check"}, &out, &errw); rc != 1 || !strings.Contains(errw.String(), "min_tier") {
		t.Fatalf("rc = %d, stderr %q; want the tier floor refused", rc, errw.String())
	}
}

func TestPolicyUsageIsAUsageError(t *testing.T) {
	for _, args := range [][]string{nil, {"list"}, {"check", "extra"}} {
		var out, errw bytes.Buffer
		if rc := runPolicy(args, &out, &errw); rc != 2 {
			t.Errorf("runPolicy(%v) rc = %d, want 2", args, rc)
		}
	}
}
