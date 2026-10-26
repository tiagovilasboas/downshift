// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// countingSource records every Discover call. It is the fake listing source.
type countingSource struct {
	harness string
	calls   atomic.Int32
}

func (c *countingSource) Harness() string { return c.harness }
func (c *countingSource) Name() string    { return "counting test source" }
func (c *countingSource) Discover(context.Context) ([]Model, error) {
	c.calls.Add(1)
	return []Model{{ID: "test-model-a"}, {ID: "test-model-b"}}, nil
}

// spawnCounter counts Spawn calls and never waits.
type spawnCounter struct{ n atomic.Int32 }

func (s *spawnCounter) spawn(string) error { s.n.Add(1); return nil }

func freshCache(t *testing.T, harness string, fetched time.Time) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "discovered.json")
	if err := Merge(path, map[string]Entry{harness: {Source: "test", FetchedAt: fetched, Ranked: Ranked{IDs: []string{"test-model-a"}}}}); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRefresherStaleEntryStartsOneRoundThatWritesTheCache(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := freshCache(t, "kirocrew", time.Now().Add(-48*time.Hour))
	done := make(chan struct{})
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: 24 * time.Hour, Spawn: func(h string) error {
		// The detached round, simulated in-process: run the source and write.
		go func() {
			defer close(done)
			out := Run(context.Background(), []Source{src}, time.Second)
			updates := map[string]Entry{}
			for _, o := range out {
				if o.Err == nil {
					updates[h] = Entry{Source: o.Source.Name(), FetchedAt: time.Now(), Models: o.Models}
				}
			}
			_ = Merge(cache, updates)
		}()
		return nil
	}}
	started, err := r.Trigger("kirocrew")
	if err != nil || !started {
		t.Fatalf("stale entry must start a round: started=%v err=%v", started, err)
	}
	<-done
	if n := src.calls.Load(); n != 1 {
		t.Fatalf("source ran %d times, want 1", n)
	}
	if e := Read(cache).Harnesses["kirocrew"]; time.Since(e.FetchedAt) > time.Minute {
		t.Fatalf("round did not refresh the entry: %+v", e)
	}
	// The next spawn sees a fresh entry and starts nothing.
	again, _ := r.Trigger("kirocrew")
	if again || src.calls.Load() != 1 {
		t.Fatalf("fresh entry triggered a round (started=%v calls=%d)", again, src.calls.Load())
	}
}

func TestRefresherFreshEntryStartsNothing(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := freshCache(t, "kirocrew", time.Now().Add(-time.Hour))
	var sc spawnCounter
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: 24 * time.Hour, Spawn: sc.spawn}
	if started, err := r.Trigger("kirocrew"); started || err != nil {
		t.Fatalf("fresh entry: started=%v err=%v", started, err)
	}
	if sc.n.Load() != 0 {
		t.Fatalf("spawn called %d times for a fresh entry", sc.n.Load())
	}
}

func TestRefresherZeroTTLNeverExpires(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := freshCache(t, "kirocrew", time.Now().Add(-365*24*time.Hour))
	var sc spawnCounter
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: 0, Spawn: sc.spawn}
	if started, _ := r.Trigger("kirocrew"); started || sc.n.Load() != 0 {
		t.Fatal("TTL 0 entry must never be stale")
	}
}

func TestRefresherMissingCacheDoesNotRunTheSourceInTheCallersPath(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := filepath.Join(t.TempDir(), "discovered.json")
	release := make(chan struct{})
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: 24 * time.Hour, Spawn: func(string) error {
		// Spawn returns at once; the round itself waits for release, so any
		// synchronous work on the caller's side would show up as a call here.
		go func() { <-release }()
		return nil
	}}
	start := time.Now()
	started, err := r.Trigger("kirocrew")
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("Trigger took %v; the caller must not wait for the round", elapsed)
	}
	if err != nil || !started {
		t.Fatalf("missing cache must start a round: started=%v err=%v", started, err)
	}
	if src.calls.Load() != 0 {
		t.Fatal("the source ran in the caller's path")
	}
	close(release)
	if e, ok := Read(cache).Harnesses["kirocrew"]; ok {
		t.Fatalf("nothing should be cached before the round writes: %+v", e)
	}
}

func TestRefresherConcurrentTriggersStartOneRound(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := filepath.Join(t.TempDir(), "discovered.json")
	var sc spawnCounter
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: 24 * time.Hour, Spawn: sc.spawn}
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = r.Trigger("kirocrew")
		}()
	}
	wg.Wait()
	if n := sc.n.Load(); n != 1 {
		t.Fatalf("%d concurrent triggers started %d rounds, want 1", 16, n)
	}
}

func TestRefresherStaleMarkerIsReplacedAfterRetry(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := filepath.Join(t.TempDir(), "discovered.json")
	marker := filepath.Join(filepath.Dir(cache), "discovery-kirocrew.refresh")
	if err := os.WriteFile(marker, []byte("abandoned\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * DefaultRefreshRetry)
	if err := os.Chtimes(marker, old, old); err != nil {
		t.Fatal(err)
	}
	var sc spawnCounter
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: time.Hour, Spawn: sc.spawn}
	if started, err := r.Trigger("kirocrew"); !started || err != nil {
		t.Fatalf("abandoned marker must not block forever: started=%v err=%v", started, err)
	}
}

func TestRefresherFailedSpawnKeepsMarkerSoItIsNotRetriedEveryHook(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := filepath.Join(t.TempDir(), "discovered.json")
	var n atomic.Int32
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: time.Hour, Spawn: func(string) error {
		n.Add(1)
		return errors.New("exec refused")
	}}
	if started, err := r.Trigger("kirocrew"); started || err == nil {
		t.Fatalf("a failed spawn must report an error, got started=%v err=%v", started, err)
	}
	if started, _ := r.Trigger("kirocrew"); started || n.Load() != 1 {
		t.Fatalf("retry after a failed spawn within the window (spawns=%d)", n.Load())
	}
}

func TestRefresherHarnessWithoutSourceNeverTriggers(t *testing.T) {
	src := &countingSource{harness: "kirocrew"}
	cache := filepath.Join(t.TempDir(), "discovered.json")
	var sc spawnCounter
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: time.Hour, Spawn: sc.spawn}
	for _, h := range []string{"claude-code", "opencode", "antigravity"} {
		if started, _ := r.Trigger(h); started {
			t.Fatalf("%s has no listing source and must not trigger", h)
		}
	}
	if sc.n.Load() != 0 {
		t.Fatal("spawn called for a harness without a source")
	}
}

func TestRefresherRejectsUnsafeHarnessNames(t *testing.T) {
	src := &countingSource{harness: "../escape"}
	cache := filepath.Join(t.TempDir(), "discovered.json")
	var sc spawnCounter
	r := Refresher{CachePath: cache, Sources: []Source{src}, TTL: time.Hour, Spawn: sc.spawn}
	if started, _ := r.Trigger("../escape"); started || sc.n.Load() != 0 {
		t.Fatal("a harness id outside the safe pattern must never name a marker file")
	}
}
