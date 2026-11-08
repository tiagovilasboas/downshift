// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// End to end: `setup --dry-run` from HOME plans no "project" policy (that path
// is the user policy) and says why; from a project it still plans one.
func TestSetupFromHomeSkipsProjectPolicy(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv(paths.EnvStateDir, "")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	userPolicy, err := policy.UserPath()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".downshift", "policy.json"); userPolicy != want {
		t.Fatalf("precondition: user policy %q, want %q", userPolicy, want)
	}
	run := func(cwd string) string {
		t.Chdir(cwd)
		var out, errw bytes.Buffer
		args := []string{"claude-code", "--dry-run", "--no-plugin", "--no-mcp", "--no-mods", "--allow-unsafe-binary"}
		if code := runSetup(args, strings.NewReader(""), &out, &errw); code != 0 {
			t.Fatalf("setup from %s exited %d: %s%s", cwd, code, out.String(), errw.String())
		}
		return out.String()
	}

	got := run(home)
	if !strings.Contains(got, "policy: skipped:") || !strings.Contains(got, "is your user policy file") {
		t.Errorf("setup from HOME did not say it skipped the project policy:\n%s", got)
	}
	if strings.Contains(got, "policy.json would change") {
		t.Errorf("setup from HOME planned the user policy file as a project policy:\n%s", got)
	}
	if _, err := os.Stat(userPolicy); !os.IsNotExist(err) {
		t.Errorf("dry run wrote %s: %v", userPolicy, err)
	}

	project := filepath.Join(home, "src", "app")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	got = run(project)
	if strings.Contains(got, "policy: skipped:") || !strings.Contains(got, "policy.json would change") {
		t.Errorf("setup from a project should plan its policy:\n%s", got)
	}
}
