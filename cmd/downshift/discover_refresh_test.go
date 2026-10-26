// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// No test may start a real `downshift models discover` process.
func init() {
	discoverRefreshSpawn = func(string) error { return nil }
}

// countDiscoverSpawns swaps the spawn for a counter for the test's duration.
func countDiscoverSpawns(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	old := discoverRefreshSpawn
	discoverRefreshSpawn = func(string) error { n.Add(1); return nil }
	t.Cleanup(func() { discoverRefreshSpawn = old })
	return &n
}

func TestHookStaleOrMissingCacheStartsOneBackgroundRefresh(t *testing.T) {
	t.Setenv("DOWNSHIFT_DISCOVERY", "")
	t.Setenv("DOWNSHIFT_DISCOVERY_TTL", "")
	t.Setenv("DOWNSHIFT_DISCOVERED", filepath.Join(t.TempDir(), "discovered.json"))
	n := countDiscoverSpawns(t)
	maybeRefreshDiscovery("kirocrew")
	maybeRefreshDiscovery("kirocrew") // second hook while the first round is in flight
	if got := n.Load(); got != 1 {
		t.Fatalf("two hooks started %d rounds, want 1", got)
	}
}

func TestHookFreshCacheStartsNoRefresh(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	body := `{"version":1,"harnesses":{"kirocrew":{"source":"test","fetched_at":"` +
		time.Now().UTC().Format(time.RFC3339) + `","ordered":["claude-haiku-4-5"]}}}`
	if err := os.WriteFile(cache, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_DISCOVERED", cache)
	t.Setenv("DOWNSHIFT_DISCOVERY_TTL", "")
	n := countDiscoverSpawns(t)
	maybeRefreshDiscovery("kirocrew")
	if n.Load() != 0 {
		t.Fatalf("fresh cache started %d rounds", n.Load())
	}
}

func TestHookDiscoveryOffOrInvalidTTLNeverSpawns(t *testing.T) {
	t.Setenv("DOWNSHIFT_DISCOVERED", filepath.Join(t.TempDir(), "discovered.json"))
	for _, tc := range []struct{ name, off, ttl string }{
		{"off", "off", ""},
		{"invalid ttl", "", "soon"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DOWNSHIFT_DISCOVERY", tc.off)
			t.Setenv("DOWNSHIFT_DISCOVERY_TTL", tc.ttl)
			n := countDiscoverSpawns(t)
			maybeRefreshDiscovery("kirocrew")
			if n.Load() != 0 {
				t.Fatalf("%s started %d rounds", tc.name, n.Load())
			}
		})
	}
}

func TestHookWithoutCacheHoldsWithoutWaitingForTheRound(t *testing.T) {
	t.Setenv("DOWNSHIFT_DISCOVERED", filepath.Join(t.TempDir(), "discovered.json"))
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent.json"))
	t.Setenv("DOWNSHIFT_DISCOVERY_TTL", "")
	t.Setenv("DOWNSHIFT_DISCOVERY", "")
	// The spawn returns at once, but the round it starts would block: a hook
	// that waited on it would exceed the bound below.
	old := discoverRefreshSpawn
	discoverRefreshSpawn = func(string) error { go time.Sleep(time.Hour); return nil }
	t.Cleanup(func() { discoverRefreshSpawn = old })

	frontier := cmdCat.ModelFor("kirocrew", core.TierFrontier).ID
	small := cmdCat.ModelFor("kirocrew", core.TierSmall).ID
	stdin := `{"tool_name":"spawn_run","tool_input":{"task":"` + kirocrewTrivialTask + `","model":"` + frontier + `"}}`
	start := time.Now()
	var rc int
	stdout := captureStdout(func() {
		rc = harnessRegistry["kirocrew"].Hook(strings.NewReader(stdin), cmdCat)
	})
	if elapsed := time.Since(start); elapsed > 1500*time.Millisecond {
		t.Fatalf("hook took %v; the 1.5 s hook budget is exceeded", elapsed)
	}
	if rc != 0 {
		t.Fatalf("exit %d, want fail-open 0", rc)
	}
	if strings.Contains(stdout, small) {
		t.Fatalf("no cache and no payload list must hold, but the hook rewrote to %s:\n%s", small, stdout)
	}
}
