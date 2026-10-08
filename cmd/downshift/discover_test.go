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
