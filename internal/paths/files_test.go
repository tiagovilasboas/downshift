// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package paths_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/testenv"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// The file names are a contract with the docs and with users' backups; each
// accessor must put its file directly under the effective state directory,
// including when DOWNSHIFT_STATE_DIR moves it.
func TestFileAccessorsLiveUnderTheStateDir(t *testing.T) {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	t.Setenv("DOWNSHIFT_SESSION_MODELS", "")
	for name, fn := range map[string]func() (string, error){
		"events.jsonl":        paths.EventsPath,
		"agents.jsonl":        paths.AgentsPath,
		"catalog.json":        paths.CatalogPath,
		"session-models.json": paths.SessionModelsPath,
		"loop-events.jsonl":   paths.LoopEventsPath,
		"dci-events.jsonl":    paths.DCIEventsPath,
		"weights.json":        paths.WeightsPath,
		"adapt-memory.json":   paths.AdaptMemoryPath,
	} {
		got, err := fn()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := filepath.Join(state, name); got != want {
			t.Errorf("%s: got %q, want %q", name, got, want)
		}
	}
}

func TestJoinAndStateDirFollowResolve(t *testing.T) {
	state := t.TempDir()
	t.Setenv(paths.EnvStateDir, state)
	if dir, err := paths.StateDir(); err != nil || dir != state {
		t.Fatalf("StateDir = %q, %v", dir, err)
	}
	got, err := paths.Join("a", "b", "c.json")
	if err != nil || got != filepath.Join(state, "a", "b", "c.json") {
		t.Fatalf("Join = %q, %v", got, err)
	}
}

func TestSessionModelsPathEnvOverrideWinsOverTheStateDir(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv("DOWNSHIFT_SESSION_MODELS", "/custom/models.json")
	got, err := paths.SessionModelsPath()
	if err != nil || got != "/custom/models.json" {
		t.Fatalf("got %q, %v", got, err)
	}
	if d := paths.DisplaySessionModels(); d != "/custom/models.json" {
		t.Fatalf("display = %q", d)
	}
}

func TestDisplayStateDirUsesTildeUnderHome(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	t.Setenv("DOWNSHIFT_SESSION_MODELS", "")

	t.Setenv(paths.EnvStateDir, "")
	if got := paths.DisplayStateDir(); got != "~/"+paths.DirName {
		t.Errorf("default = %q", got)
	}
	if got := paths.DisplaySessionModels(); got != "~/"+paths.DirName+"/session-models.json" {
		t.Errorf("session models = %q", got)
	}

	t.Setenv(paths.EnvStateDir, filepath.Join(home, "custom", "state"))
	if got := paths.DisplayStateDir(); got != "~/custom/state" {
		t.Errorf("under home = %q", got)
	}

	outside := t.TempDir() // a sibling of home, not under it
	t.Setenv(paths.EnvStateDir, outside)
	if got := paths.DisplayStateDir(); got != outside || strings.HasPrefix(got, "~") {
		t.Errorf("outside home = %q, want the absolute path", got)
	}
}
