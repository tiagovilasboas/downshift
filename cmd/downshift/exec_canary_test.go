// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// A canary placed in an argument, the working directory, the environment and
// the program output must not appear anywhere in the state directory.
func TestRunExec_CanaryNeverReachesState(t *testing.T) {
	const canary = "CANARY-7f3a9c"
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	t.Setenv("CANARY_ENV", canary)
	work := filepath.Join(t.TempDir(), canary)
	if err := os.Mkdir(work, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(work)
	bin := t.TempDir()
	script := "#!/bin/sh\necho " + canary + "-out\necho " + canary + "-err >&2\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	var out, errb bytes.Buffer
	runExec([]string{"--", "go", "test", "./" + canary + "/..."}, strings.NewReader(""), &out, &errb)
	runExec([]string{"--", "rm", "-rf", canary}, strings.NewReader(""), &out, &errb) // refused

	err := filepath.Walk(state, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		if strings.Contains(string(data), "CANARY") || strings.Contains(p, "CANARY") {
			t.Errorf("canary leaked into %s: %s", p, data)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(state, execwrap.EventsFileName)); err != nil {
		t.Fatalf("expected an exec log: %v", err)
	}
}

// Under the recursion guard the wrapper does not compact.
func TestRunExec_DepthGuardDisablesCompaction(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv(execwrap.DepthEnv, "1")
	bin := t.TempDir()
	var suite strings.Builder
	for i := 0; i < 8; i++ {
		suite.WriteString("ok  \tsecret/pkg/p\t0.1s\n")
	}
	script := "#!/bin/sh\nprintf '" + strings.ReplaceAll(suite.String(), "\t", "\\t") + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	var out, errb bytes.Buffer
	if rc := runExec([]string{"--", "go", "test"}, strings.NewReader(""), &out, &errb); rc != 0 || out.String() != suite.String() {
		t.Fatalf("rc=%d out=%q", rc, out.String())
	}
}

func TestShellRewrite_DepthGuardBlocksHook(t *testing.T) {
	withExec(t, true, map[string]string{execwrap.DepthEnv: "1"}, true)
	if d := shellWrapDecision("claude-code", "go test ./...", func(string) bool { return false }); d.Rewrite {
		t.Fatalf("decision = %+v", d)
	}
}
