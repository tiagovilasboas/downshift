// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Never launch a real refresh from any test in this package: the spawner is a
// no-op unless a test installs its own fake.
func init() { scoreRefreshSpawn = func() error { return nil } }

// scoreHookPayload is accepted by every registered harness hook (it may parse
// as a non-spawn event; the refresh trigger does not depend on the decision).
const scoreHookPayload = `{"hook_event_name":"preToolUse","tool_name":"Task","tool_input":{"task":"rename the userId variable"}}`

func scoreFakeSpawn(t *testing.T, err error) *int {
	t.Helper()
	n := 0
	old := scoreRefreshSpawn
	scoreRefreshSpawn = func() error { n++; return err }
	t.Cleanup(func() { scoreRefreshSpawn = old })
	return &n
}

func scoreStateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(dir, "events.jsonl"))
	t.Setenv("DOWNSHIFT_SCORES", "")
	return dir
}

func runAllHarnessHooks(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for name, e := range harnessRegistry {
		if e.Hook == nil {
			continue
		}
		out[name] = captureStdout(func() { e.Hook(strings.NewReader(scoreHookPayload), cmdCat) })
	}
	if len(out) == 0 {
		t.Fatal("no harness hooks registered")
	}
	return out
}

func writeScoreCache(t *testing.T, dir string, age time.Duration) {
	t.Helper()
	p := filepath.Join(dir, scoreCacheFile)
	if err := os.WriteFile(p, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ts := time.Now().Add(-age)
	if err := os.Chtimes(p, ts, ts); err != nil {
		t.Fatal(err)
	}
}

func assertSameOutputs(t *testing.T, got, want map[string]string) {
	t.Helper()
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s: output changed:\n got %q\nwant %q", k, got[k], w)
		}
	}
}

func TestScoreRefresh_MissingCacheSpawnsOncePerHarnessAndLeavesOutputUnchanged(t *testing.T) {
	dir := scoreStateDir(t)
	t.Setenv("DOWNSHIFT_SCORES", "off")
	want := runAllHarnessHooks(t)

	t.Setenv("DOWNSHIFT_SCORES", "")
	for name, e := range harnessRegistry {
		if e.Hook == nil {
			continue
		}
		_ = os.Remove(filepath.Join(dir, scoreLockFile))
		n := scoreFakeSpawn(t, nil)
		got := captureStdout(func() { e.Hook(strings.NewReader(scoreHookPayload), cmdCat) })
		if got != want[name] {
			t.Errorf("%s: hook output changed by refresh trigger:\n got %q\nwant %q", name, got, want[name])
		}
		if *n != 1 {
			t.Errorf("%s: spawns = %d, want 1 (missing cache)", name, *n)
		}
		// A second parallel hook sees the lock and must not spawn again.
		got = captureStdout(func() { e.Hook(strings.NewReader(scoreHookPayload), cmdCat) })
		if *n != 1 {
			t.Errorf("%s: spawns after second hook = %d, want 1 (lock)", name, *n)
		}
		if got != want[name] {
			t.Errorf("%s: second hook output changed", name)
		}
	}
}

func TestScoreRefresh_StaleCacheSpawnsOnceAcrossHarnesses(t *testing.T) {
	dir := scoreStateDir(t)
	writeScoreCache(t, dir, 25*time.Hour)
	n := scoreFakeSpawn(t, nil)
	runAllHarnessHooks(t)
	if *n != 1 {
		t.Fatalf("spawns = %d, want exactly 1 across all harness hooks (shared lock)", *n)
	}
}

func TestScoreRefresh_FreshCacheOffAndSpawnErrorKeepOutput(t *testing.T) {
	dir := scoreStateDir(t)
	t.Setenv("DOWNSHIFT_SCORES", "off")
	want := runAllHarnessHooks(t)

	t.Run("fresh cache", func(t *testing.T) {
		t.Setenv("DOWNSHIFT_SCORES", "")
		writeScoreCache(t, dir, time.Hour)
		n := scoreFakeSpawn(t, nil)
		got := runAllHarnessHooks(t)
		if *n != 0 {
			t.Errorf("spawns = %d, want 0", *n)
		}
		assertSameOutputs(t, got, want)
	})
	t.Run("off", func(t *testing.T) {
		_ = os.Remove(filepath.Join(dir, scoreCacheFile))
		t.Setenv("DOWNSHIFT_SCORES", "OFF")
		n := scoreFakeSpawn(t, nil)
		got := runAllHarnessHooks(t)
		if *n != 0 {
			t.Errorf("spawns = %d, want 0", *n)
		}
		assertSameOutputs(t, got, want)
	})
	t.Run("spawn error", func(t *testing.T) {
		_ = os.Remove(filepath.Join(dir, scoreCacheFile))
		_ = os.Remove(filepath.Join(dir, scoreLockFile))
		t.Setenv("DOWNSHIFT_SCORES", "")
		n := scoreFakeSpawn(t, errors.New("boom"))
		got := runAllHarnessHooks(t)
		if *n != 1 {
			t.Errorf("spawns = %d, want 1 (error keeps the lock)", *n)
		}
		assertSameOutputs(t, got, want)
	})
}

func TestScoreRefresh_StaleLockIsReplaced(t *testing.T) {
	dir := scoreStateDir(t)
	lock := filepath.Join(dir, scoreLockFile)
	if err := os.WriteFile(lock, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-11 * time.Minute)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	n := scoreFakeSpawn(t, nil)
	if !maybeRefreshScores() || *n != 1 {
		t.Fatalf("stale lock must be replaced and refresh started (spawns=%d)", *n)
	}
	if maybeRefreshScores() || *n != 1 {
		t.Fatalf("young lock must block a second spawn (spawns=%d)", *n)
	}
}

func TestScoreRefresh_PanickingSpawnerFailsOpen(t *testing.T) {
	scoreStateDir(t)
	old := scoreRefreshSpawn
	scoreRefreshSpawn = func() error { panic("boom") }
	t.Cleanup(func() { scoreRefreshSpawn = old })
	if maybeRefreshScores() {
		t.Fatal("panic must not report started")
	}
}

func TestDoctor_ScoreStatus(t *testing.T) {
	dir := scoreStateDir(t)
	n := scoreFakeSpawn(t, nil)
	var b bytes.Buffer
	runDoctor(&b, cmdCat)
	if !strings.Contains(b.String(), "scores: missing") || *n != 1 {
		t.Errorf("missing cache: want status and one refresh (n=%d):\n%s", *n, b.String())
	}
	b.Reset()
	writeScoreCache(t, dir, time.Hour)
	runDoctor(&b, cmdCat)
	if !strings.Contains(b.String(), "scores: fresh") || *n != 1 {
		t.Errorf("fresh cache: want fresh and no new refresh (n=%d):\n%s", *n, b.String())
	}
	b.Reset()
	t.Setenv("DOWNSHIFT_SCORES", "off")
	runDoctor(&b, cmdCat)
	if !strings.Contains(b.String(), "scores: off") {
		t.Errorf("off: %s", b.String())
	}
}
