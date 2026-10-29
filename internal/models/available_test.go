// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package models

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

// fakeMeta is a catalog with made-up ids; the resolver tests name no real model.
type fakeMeta struct {
	rows       map[string]core.Tier
	efforts    map[string][]string
	candidates []string
	explicit   map[string]bool
}

func (f fakeMeta) LookupByID(h, id string) (core.Model, bool) {
	t, ok := f.rows[id]
	return core.Model{ID: id, Tier: t, Harness: h}, ok
}
func (f fakeMeta) AllowedEfforts(_, id string) []string { return f.efforts[id] }
func (f fakeMeta) SessionCandidates(string) []string    { return f.candidates }
func (f fakeMeta) IsExplicitOnly(_, id string) bool     { return f.explicit[id] }

func newFake() fakeMeta {
	return fakeMeta{
		rows:       map[string]core.Tier{"fast-1": core.TierSmall, "strong-1": core.TierFrontier, "cand-mid": core.TierMid, "cand-fast": core.TierSmall, "cand-only": core.TierFrontier},
		efforts:    map[string][]string{"fast-1": {"low", "high"}},
		explicit:   map[string]bool{"cand-only": true},
		candidates: []string{"cand-fast", "cand-mid", "cand-only", "cand-fast", "cand-ghost"},
	}
}

func stateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv(paths.EnvStateDir, dir)
	for _, k := range []string{"DOWNSHIFT_DISCOVERED", "DOWNSHIFT_SESSION_MODELS", "DOWNSHIFT_DISCOVERY", "DOWNSHIFT_DISCOVERY_TTL"} {
		t.Setenv(k, "")
	}
	return dir
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func discoveredJSON(h string, fetched time.Time, ids []string, unranked []string) string {
	b, _ := json.Marshal(map[string]any{"version": 1, "harnesses": map[string]any{
		h: map[string]any{"source": "test", "fetched_at": fetched, "ordered": ids, "unranked": unranked},
	}})
	return string(b)
}

func byID(l Listing) map[string]Available {
	m := map[string]Available{}
	for _, a := range l.Models {
		m[a.ID] = a
	}
	return m
}

func TestResolveEachSourceAlone(t *testing.T) {
	now := time.Now()
	for name, tc := range map[string]struct {
		setup  func(t *testing.T, dir string)
		opt    Options
		source string
	}{
		"payload": {func(*testing.T, string) {}, Options{Payload: &[]string{"fast-1", "x-9"}}, SourcePayload},
		"discovery": {func(t *testing.T, d string) {
			write(t, d, "discovered.json", discoveredJSON("cursor", now, []string{"fast-1", "x-9"}, nil))
		}, Options{}, SourceDiscovery},
		"file": {func(t *testing.T, d string) {
			write(t, d, "session-models.json", `{"cursor":["fast-1","x-9"]}`)
		}, Options{}, SourceFile},
	} {
		t.Run(name, func(t *testing.T) {
			dir := stateDir(t)
			tc.setup(t, dir)
			l, err := Resolve("cursor", newFake(), tc.opt)
			if err != nil {
				t.Fatal(err)
			}
			got := byID(l)
			if len(got) != 2 || got["fast-1"].Source != tc.source || got["x-9"].Source != tc.source {
				t.Fatalf("models = %+v", l.Models)
			}
			if got["fast-1"].Tier != "small" || !reflect.DeepEqual(got["fast-1"].Efforts, []string{"low", "high"}) {
				t.Errorf("catalog metadata not applied: %+v", got["fast-1"])
			}
			if l.Fallback {
				t.Error("a real source is not a fallback")
			}
		})
	}
}

func TestResolvePriorityPayloadThenDiscoveryThenFile(t *testing.T) {
	dir := stateDir(t)
	write(t, dir, "discovered.json", discoveredJSON("cursor", time.Now(), []string{"fast-1"}, nil))
	write(t, dir, "session-models.json", `{"cursor":["strong-1"]}`)
	l, _ := Resolve("cursor", newFake(), Options{Payload: &[]string{"x-9"}})
	if len(l.Models) != 1 || l.Models[0].ID != "x-9" {
		t.Fatalf("payload must win: %+v", l.Models)
	}
	l, _ = Resolve("cursor", newFake(), Options{})
	if len(l.Models) != 1 || l.Models[0].ID != "fast-1" || l.CacheWrittenAt.IsZero() {
		t.Fatalf("discovery must beat the file: %+v", l)
	}
}

func TestResolveUnknownTierIsListedAndFlagged(t *testing.T) {
	dir := stateDir(t)
	write(t, dir, "discovered.json", discoveredJSON("cursor", time.Now(), []string{"fast-1", "mystery"}, []string{"unranked-1", "fast-1"}))
	l, _ := Resolve("cursor", newFake(), Options{})
	got := byID(l)
	if len(l.Models) != 3 {
		t.Fatalf("models = %+v", l.Models)
	}
	for _, id := range []string{"mystery", "unranked-1"} {
		if !got[id].Untiered || got[id].Tier != TierUnknown {
			t.Errorf("%s must be untiered: %+v", id, got[id])
		}
	}
	if got["fast-1"].Untiered {
		t.Error("a catalog tier clears the flag")
	}
}

func TestResolveSourceMetadataBeatsCatalog(t *testing.T) {
	dir := stateDir(t)
	write(t, dir, "discovered.json", `{"version":1,"harnesses":{"cursor":{"fetched_at":"`+time.Now().Format(time.RFC3339)+`","ordered":["fast-1"],"model_metadata":[{"id":"fast-1","tier":"Frontier","efforts":["max"]}]}}}`)
	l, _ := Resolve("cursor", newFake(), Options{})
	if a := l.Models[0]; a.Tier != "frontier" || !reflect.DeepEqual(a.Efforts, []string{"max"}) {
		t.Fatalf("source metadata must win: %+v", a)
	}
}

func TestResolveCatalogNeverAddsAnID(t *testing.T) {
	dir := stateDir(t)
	write(t, dir, "session-models.json", `{"claude-code":["x-9"]}`)
	l, _ := Resolve("claude-code", newFake(), Options{})
	if len(l.Models) != 1 || l.Models[0].ID != "x-9" || l.Fallback {
		t.Fatalf("catalog candidates must not join a real list: %+v", l)
	}
	l, _ = Resolve("cursor", newFake(), Options{})
	if len(l.Models) != 0 {
		t.Fatalf("no source and no stand-in means no models: %+v", l)
	}
}

func TestResolveClaudeCodeFallbackIsFlagged(t *testing.T) {
	stateDir(t)
	l, err := Resolve("claude-code", newFake(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !l.Fallback || len(l.Models) != 2 {
		t.Fatalf("want deduped automatic candidates only: %+v", l)
	}
	for _, a := range l.Models {
		if a.Source != SourceCatalogFallback {
			t.Errorf("source = %s", a.Source)
		}
	}
}

func TestResolveEmptyPayloadIsAuthoritative(t *testing.T) {
	dir := stateDir(t)
	write(t, dir, "session-models.json", `{"cursor":["fast-1"]}`)
	l, _ := Resolve("cursor", newFake(), Options{Payload: &[]string{}})
	if len(l.Models) != 0 {
		t.Fatalf("an empty payload list holds routing: %+v", l.Models)
	}
}

func TestResolveUnknownHarness(t *testing.T) {
	if _, err := Resolve("nope", newFake(), Options{}); err != ErrUnknownHarness {
		t.Fatalf("err = %v", err)
	}
}
