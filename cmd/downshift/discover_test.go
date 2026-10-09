// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/discovery"
)

type fakeSource struct {
	harness string
	models  []discovery.Model
	err     error
}

func (f fakeSource) Harness() string { return f.harness }
func (f fakeSource) Name() string    { return "fake " + f.harness }
func (f fakeSource) Discover(context.Context) ([]discovery.Model, error) {
	return f.models, f.err
}

func TestDiscoverWritesRankedCacheAndKeepsFailedHarness(t *testing.T) {
	cat := catalog.Load()
	cache := filepath.Join(t.TempDir(), "discovered.json")
	good := fakeSource{harness: "claude-code", models: []discovery.Model{
		{ID: "claude-opus-5-5"}, {ID: "claude-haiku-5-5"}, {ID: "claude-sonnet-5-5"},
	}}
	var out, errOut bytes.Buffer
	if code := discoverWith(cat, []discovery.Source{good}, cache, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	f := discovery.Read(cache)
	want := []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"}
	got := f.Harnesses["claude-code"].IDs
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("ordered = %v, want %v", got, want)
	}

	// A later failure for that harness must leave the previous entry in place.
	bad := fakeSource{harness: "claude-code", err: errors.New("offline")}
	other := fakeSource{harness: "kirocrew", models: []discovery.Model{{ID: "claude-haiku-4.5"}}}
	out.Reset()
	if code := discoverWith(cat, []discovery.Source{bad, other}, cache, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	f = discovery.Read(cache)
	if len(f.Harnesses["claude-code"].IDs) != 3 || len(f.Harnesses["kirocrew"].IDs) != 1 {
		t.Fatalf("failed harness lost or new one missing: %+v", f.Harnesses)
	}
	if !strings.Contains(out.String(), "skipped: offline") {
		t.Errorf("report must say why a source was skipped:\n%s", out.String())
	}
}

func TestDiscoverDryRunWritesNothing(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	src := fakeSource{harness: "codex", models: []discovery.Model{{ID: "gpt-6-sol"}}}
	var out, errOut bytes.Buffer
	if code := discoverWith(catalog.Load(), []discovery.Source{src}, cache, []string{"--dry-run"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if _, err := os.Stat(cache); err == nil {
		t.Fatal("dry run wrote the cache")
	}
}

func TestDiscoverAllFailingLeavesCacheAndFails(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	src := fakeSource{harness: "codex", err: errors.New("no file")}
	var out, errOut bytes.Buffer
	if code := discoverWith(catalog.Load(), []discovery.Source{src}, cache, nil, &out, &errOut); code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if _, err := os.Stat(cache); err == nil {
		t.Fatal("nothing discovered must not create a cache")
	}
}

func TestDiscoverHarnessFilter(t *testing.T) {
	a := fakeSource{harness: "codex", models: []discovery.Model{{ID: "gpt-6-sol"}}}
	b := fakeSource{harness: "kirocrew", models: []discovery.Model{{ID: "claude-haiku-4.5"}}}
	cache := filepath.Join(t.TempDir(), "d.json")
	var out, errOut bytes.Buffer
	if code := discoverWith(catalog.Load(), []discovery.Source{a, b}, cache, []string{"--harness=codex"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	f := discovery.Read(cache)
	if _, ok := f.Harnesses["kirocrew"]; ok || len(f.Harnesses["codex"].IDs) == 0 {
		t.Fatalf("filter ignored: %+v", f.Harnesses)
	}
	if code := discoverWith(catalog.Load(), []discovery.Source{a}, cache, []string{"--harness=nope"}, &out, &errOut); code != 2 {
		t.Fatalf("unknown harness exit %d, want 2", code)
	}
}

func TestDiscoverEmptySourcePreservesCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	previous := discovery.Entry{Source: "previous", Ranked: discovery.Ranked{IDs: []string{"model-existing"}}}
	if err := discovery.Merge(cache, map[string]discovery.Entry{"cursor": previous}); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(cache)
	var out, errOut bytes.Buffer
	if code := discoverWith(catalog.Load(), []discovery.Source{fakeSource{harness: "cursor"}}, cache, []string{"--json"}, &out, &errOut); code != 1 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	after, _ := os.ReadFile(cache)
	if !bytes.Equal(before, after) {
		t.Fatal("empty discovery replaced valid cache")
	}
	if !strings.Contains(errOut.String(), "source returned no models") {
		t.Fatalf("JSON mode hid failure: %s", errOut.String())
	}
}

func TestDiscoverDoesNotSilentlyDropUnsupportedRequestedHarness(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	src := fakeSource{harness: "cursor", models: []discovery.Model{{ID: "example-model"}}}
	var out, errOut bytes.Buffer
	if code := discoverWith(catalog.Load(), []discovery.Source{src}, cache, []string{"--harness=cursor,antigravity"}, &out, &errOut); code != 2 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "antigravity") {
		t.Fatalf("missing unsupported harness: %s", errOut.String())
	}
	if _, err := os.Stat(cache); err == nil {
		t.Fatal("unsupported requested source still wrote cache")
	}
}

func TestDiscoverCursorCLIStoresOnlyItsReportedModels(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "cursor-agent")
	fixture := "#!/bin/sh\n[ \"$#\" = 1 ] && [ \"$1\" = models ] || exit 9\nprintf '%s\\n' 'Available models' 'future-model - Future Model' 'Tip: use --model <id>'\n"
	if err := os.WriteFile(bin, []byte(fixture), 0o700); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(dir, "discovered.json")
	var out, errOut bytes.Buffer
	src := discovery.CursorCLI{Bin: bin}
	if code := discoverWith(catalog.Load(), []discovery.Source{src}, cache, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	got := discovery.Read(cache).Harnesses["cursor"]
	if len(got.Models) != 1 || got.Models[0].ID != "future-model" || len(got.IDs) != 0 {
		t.Fatalf("discovery invented routable models: %+v", got)
	}
	before, _ := os.ReadFile(cache)
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'Available models'\necho 'partial-model - Partial'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code := discoverWith(catalog.Load(), []discovery.Source{src}, cache, nil, &out, &errOut); code != 1 {
		t.Fatalf("exit %d on incomplete listing", code)
	}
	after, _ := os.ReadFile(cache)
	if !bytes.Equal(before, after) {
		t.Fatal("incomplete Cursor output replaced the complete cached listing")
	}
}
