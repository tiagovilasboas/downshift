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
)

func setupModelsEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("DOWNSHIFT_DISCOVERED", "")
	t.Setenv("DOWNSHIFT_SESSION_MODELS", "")
	t.Setenv("DOWNSHIFT_DISCOVERY", "")
	t.Setenv("DOWNSHIFT_DISCOVERY_TTL", "")
	return dir
}

func runSetupModels(dir string, targets ...string) string {
	var out, errOut bytes.Buffer
	var ts []setupTarget
	for _, n := range targets {
		ts = append(ts, setupTarget{Name: n})
	}
	setupEnsureSessionModels(setupOptions{Out: &out, Err: &errOut, Home: dir}, ts)
	return out.String()
}

func writeDiscovered(t *testing.T, dir, harness string, at time.Time) {
	t.Helper()
	f := map[string]any{"harnesses": map[string]any{harness: map[string]any{
		"fetched_at": at, "ordered": []string{"m-small", "m-big"},
	}}}
	raw, _ := json.Marshal(f)
	if err := os.WriteFile(filepath.Join(dir, "discovered.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSetupModels_AbsentFileStaysAbsentAndHintIsPrinted(t *testing.T) {
	dir := setupModelsEnv(t)
	out := runSetupModels(dir, "codex")
	if _, err := os.Stat(filepath.Join(dir, "session-models.json")); !os.IsNotExist(err) {
		t.Fatalf("setup must not create session-models.json: %v", err)
	}
	want := "models: no detected list for codex; run `downshift models discover --harness codex`"
	if !strings.Contains(out, want) {
		t.Fatalf("hint missing: %q", out)
	}
}

func TestSetupModels_ExistingFilePreservedByteForByte(t *testing.T) {
	dir := setupModelsEnv(t)
	p := filepath.Join(dir, "session-models.json")
	orig := []byte("{\n \"claude-code\": [\"custom-small\"],\n  \"cursor\":[\"a\"]}")
	if err := os.WriteFile(p, orig, 0o600); err != nil {
		t.Fatal(err)
	}
	writeDiscovered(t, dir, "codex", time.Now())
	runSetupModels(dir, "claude-code", "codex", "opencode")
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, orig) {
		t.Fatalf("session-models.json changed: %q", got)
	}
}

func TestSetupModels_FreshDiscoveryIsQuiet(t *testing.T) {
	dir := setupModelsEnv(t)
	writeDiscovered(t, dir, "codex", time.Now())
	if out := runSetupModels(dir, "codex"); out != "" {
		t.Fatalf("a harness with a fresh entry prints nothing: %q", out)
	}
}

func TestSetupModels_StaleDiscoveryPrintsHint(t *testing.T) {
	dir := setupModelsEnv(t)
	writeDiscovered(t, dir, "codex", time.Now().Add(-72*time.Hour))
	if out := runSetupModels(dir, "codex"); !strings.Contains(out, "--harness codex") {
		t.Fatalf("an expired entry is no detected list: %q", out)
	}
}

func TestSetupModels_FileListSilencesHint(t *testing.T) {
	dir := setupModelsEnv(t)
	if err := os.WriteFile(filepath.Join(dir, "session-models.json"), []byte(`{"codex":["a","b"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if out := runSetupModels(dir, "codex"); out != "" {
		t.Fatalf("a file list is an explicit override: %q", out)
	}
}

func TestSetupModels_ClaudeCodeStandInPrintsNothing(t *testing.T) {
	dir := setupModelsEnv(t)
	if out := runSetupModels(dir, "claude-code"); out != "" {
		t.Fatalf("the Claude Code stand-in is not a gap: %q", out)
	}
}

func TestSetupModels_DryRunAndUndoAreSilent(t *testing.T) {
	dir := setupModelsEnv(t)
	for _, o := range []setupOptions{{DryRun: true}, {Undo: true}} {
		var out bytes.Buffer
		o.Out, o.Home = &out, dir
		setupEnsureSessionModels(o, []setupTarget{{Name: "codex"}})
		if out.Len() != 0 {
			t.Fatalf("%+v printed %q", o, out.String())
		}
	}
}
