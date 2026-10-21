// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// TestShellInit_PassthroughWhenNotAllowlisted evals the real `downshift enable`
// script in a POSIX shell with stdout on a pipe (not a TTY) and checks that a
// command outside the allowlist runs as typed and exits 0, while `go test`
// still goes through the compactor. `go` is a stub so the test never runs a
// nested `go test`; git and npm are the real programs.
func TestShellInit_PassthroughWhenNotAllowlisted(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(bin, "downshift"), ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	stubLog := filepath.Join(root, "stub.log")
	stub := "#!/bin/sh\necho \"depth=${" + execwrap.DepthEnv + ":-none} args=$*\" >> \"$STUB_LOG\"\nexit 0\n"
	for _, name := range []string{"go", "docker"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(stub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	repo := filepath.Join(root, "repo")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	type step struct {
		name, command string
		wantDepth     string // stub log line for stubbed programs; "" for a real program
	}
	// `go version` and `go build` are allowlisted families today, so they reach
	// the compactor (depth=1) and must still exit 0; `docker run`, `git -C` and
	// `npm --version` are not and must run as typed (depth=none, exit 0).
	steps := []step{
		{"go version", "go version", "depth=1 args=version"},
		{"go build", "go build ./...", "depth=1 args=build ./..."},
		{"docker run passes through", "docker run alpine true", "depth=none args=run alpine true"},
		{"git -C status", "git -C " + repo + " status", ""},
		{"npm --version", "npm --version", ""},
		{"go test is compacted", "go test ./...", "depth=1 args=test ./..."},
	}
	for _, sh := range availableShells(t) {
		for _, s := range steps {
			t.Run(sh+"/"+s.name, func(t *testing.T) {
				if strings.HasPrefix(s.command, "npm") {
					if _, err := exec.LookPath("npm"); err != nil {
						t.Skip("npm not on PATH")
					}
				}
				state := t.TempDir()
				_ = os.Remove(stubLog)
				cmd := exec.Command(sh, "-c", `eval "$(downshift enable)"; `+s.command)
				cmd.Env = append(os.Environ(),
					"PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"),
					"STUB_LOG="+stubLog,
					paths.EnvStateDir+"="+state,
					execwrap.DepthEnv+"=",
					"DOWNSHIFT_COMPACTOR_ACTIVE=",
					"DOWNSHIFT_SHELL_ALIASES=",
				)
				out, err := cmd.CombinedOutput() // stdout is a pipe, so the helper is active
				if err != nil {
					t.Fatalf("%q exited with %v:\n%s", s.command, err, out)
				}
				if s.wantDepth != "" {
					got, _ := os.ReadFile(stubLog)
					if strings.TrimSpace(string(got)) != s.wantDepth {
						t.Fatalf("stub log = %q, want %q\noutput: %s", got, s.wantDepth, out)
					}
				}
				_, statErr := os.Stat(filepath.Join(state, execwrap.EventsFileName))
				if compacted := s.wantDepth != "" && strings.HasPrefix(s.wantDepth, "depth=1"); compacted != (statErr == nil) {
					t.Fatalf("exec log present=%v, want %v (stat err=%v)", statErr == nil, compacted, statErr)
				}
			})
		}
	}
}

// availableShells returns sh plus every other POSIX shell on PATH, so the
// script is checked under dash, bash and zsh where they exist.
func availableShells(t *testing.T) []string {
	t.Helper()
	shells := []string{"sh"}
	for _, s := range []string{"dash", "bash", "zsh"} {
		if _, err := exec.LookPath(s); err == nil {
			shells = append(shells, s)
		}
	}
	return shells
}
