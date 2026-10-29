// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// playbookEnv points the playbook folder at a temp state dir and returns it.
func playbookEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	return dir
}

// fakeEditor writes a script that runs body against the file it is given.
func fakeEditor(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake editor is a POSIX script")
	}
	script := filepath.Join(t.TempDir(), "editor.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", script)
}

func runPB(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := runPlaybook(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestPlaybookListShowsShippedSets(t *testing.T) {
	playbookEnv(t)
	code, out, _ := runPB(t, "list")
	if code != 0 || !strings.Contains(out, "General") || !strings.Contains(out, "Frontend") {
		t.Fatalf("list exit %d:\n%s", code, out)
	}
}

func TestPlaybookRemovedShowsInTheListing(t *testing.T) {
	playbookEnv(t)
	if code, _, errs := runPB(t, "remove", "frontend"); code != 0 {
		t.Fatalf("remove exit %d: %s", code, errs)
	}
	_, out, _ := runPB(t, "list")
	if !strings.Contains(out, "removed") {
		t.Fatalf("a removed shipped set must show as removed:\n%s", out)
	}
}

func TestPlaybookShowUnknownNameListsTheValidOnes(t *testing.T) {
	playbookEnv(t)
	code, _, errs := runPB(t, "show", "Mobile")
	if code == 0 || !strings.Contains(errs, "Available: General, Backend, Frontend") {
		t.Fatalf("exit %d, stderr %q", code, errs)
	}
}

func TestPlaybookGeneralCannotBeRemoved(t *testing.T) {
	playbookEnv(t)
	if code, _, _ := runPB(t, "remove", "general"); code == 0 {
		t.Fatal("general was removed")
	}
}

func TestPlaybookRestoreRefusesACustomSet(t *testing.T) {
	dir := playbookEnv(t)
	if err := os.MkdirAll(filepath.Join(dir, "playbooks"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "playbooks", "dados.md"), []byte("# Dados\n\n- rule\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if code, _, _ := runPB(t, "restore", "dados"); code == 0 {
		t.Fatal("restore accepted a custom set")
	}
}

func TestPlaybookAddOpensTheEditorAndSavesTheRule(t *testing.T) {
	playbookEnv(t)
	fakeEditor(t, `echo "- Use migrations for every schema change." >> "$1"`)
	code, out, errs := runPB(t, "add", "Banco de Dados")
	if code != 0 {
		t.Fatalf("add exit %d: %s", code, errs)
	}
	if !strings.Contains(out, "Banco de Dados") {
		t.Fatalf("add did not report the new set:\n%s", out)
	}
	_, shown, _ := runPB(t, "show", "banco de dados")
	if !strings.Contains(shown, "Use migrations") {
		t.Fatalf("the rule was not saved:\n%s", shown)
	}
}

func TestPlaybookEditToEmptyNoLongerActive(t *testing.T) {
	playbookEnv(t)
	fakeEditor(t, `: > "$1"`)
	code, out, _ := runPB(t, "edit", "backend")
	if code != 0 || !strings.Contains(out, "no text") {
		t.Fatalf("exit %d, out %q", code, out)
	}
	if _, found := runPBFind(t, "backend"); found {
		t.Fatal("an emptied backend is still active")
	}
}

func TestPlaybookAddRefusesAnExistingName(t *testing.T) {
	playbookEnv(t)
	if code, _, _ := runPB(t, "add", "Backend"); code == 0 {
		t.Fatal("add accepted a name that already exists")
	}
}

func TestPlaybookCheckPassesOnShippedSets(t *testing.T) {
	playbookEnv(t)
	if code, out, _ := runPB(t, "check"); code != 0 || !strings.Contains(out, "names unique") {
		t.Fatalf("check exit %d: %s", code, out)
	}
}

func runPBFind(t *testing.T, name string) (string, bool) {
	t.Helper()
	code, out, _ := runPB(t, "show", name)
	return out, code == 0
}

func TestPlaybookAddLeftWithoutTextIsNotKept(t *testing.T) {
	dir := playbookEnv(t)
	fakeEditor(t, `:`)
	code, out, _ := runPB(t, "add", "Dados")
	if code != 0 || !strings.Contains(out, "no text") {
		t.Fatalf("exit %d, out %q", code, out)
	}
	if _, err := os.Stat(filepath.Join(dir, "playbooks", "dados.md")); !os.IsNotExist(err) {
		t.Fatalf("an empty playbook file was kept: %v", err)
	}
	if code, _, _ := runPB(t, "check"); code != 0 {
		t.Fatal("check fails after an abandoned add")
	}
}
