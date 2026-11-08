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
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// The cmd tests that drive setup end to end (the flag parser, the TUI, policy
// init, the hook adapters' setup rows) share this home fixture. Its twin for
// the package's own tests is internal/setup/run_test.go.

const testBin = "/opt/ds/downshift"

type setupEnv struct {
	t    *testing.T
	home string
	tick int
}

func newSetupEnv(t *testing.T, dirs ...string) *setupEnv {
	t.Helper()
	home := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return &setupEnv{t: t, home: home}
}

func (e *setupEnv) path(rel string) string { return filepath.Join(e.home, rel) }

func (e *setupEnv) write(rel, content string) {
	e.t.Helper()
	if err := os.MkdirAll(filepath.Dir(e.path(rel)), 0o755); err != nil {
		e.t.Fatal(err)
	}
	if err := os.WriteFile(e.path(rel), []byte(content), 0o644); err != nil {
		e.t.Fatal(err)
	}
}

func (e *setupEnv) read(rel string) string {
	e.t.Helper()
	b, err := os.ReadFile(e.path(rel))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(b)
}

// run executes setup with a fixed binary path and a strictly increasing clock.
func (e *setupEnv) run(args ...string) (code int, out, errOut string) {
	e.t.Helper()
	var o, er bytes.Buffer
	opts := setup.Options{Home: e.home, Binary: testBin, SkipDiscovery: true, In: strings.NewReader(""), Out: &o, Err: &er}
	opts.Now = func() time.Time {
		e.tick++
		return time.Date(2026, 10, 13, 12, 0, 0, e.tick*1000, time.UTC)
	}
	for _, a := range args {
		switch a {
		case "--dry-run":
			opts.DryRun = true
		case "--undo":
			opts.Undo = true
		case "--yes":
			opts.Yes = true
		case "--you-should-know":
			opts.YouShouldKnow = true
		case "--no-session-policy":
			opts.NoSessionPolicy = true
		case "--no-mods":
			opts.NoMods = true
		case "--mcp":
			opts.Mcp = true
		default:
			opts.Names = append(opts.Names, a)
		}
	}
	return setupRun(opts), o.String(), er.String()
}

func parseDoc(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, s)
	}
	return m
}

// hookCommands lists every command under hooks[event] of a nested-shape file.
func hookCommands(t *testing.T, doc map[string]any, event string) []string {
	t.Helper()
	var cmds []string
	hooks, _ := doc["hooks"].(map[string]any)
	groups, _ := hooks[event].([]any)
	for _, g := range groups {
		for _, h := range g.(map[string]any)["hooks"].([]any) {
			cmds = append(cmds, h.(map[string]any)["command"].(string))
		}
	}
	return cmds
}
