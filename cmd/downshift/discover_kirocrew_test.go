// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/discovery"
)

// kiroListing is the fixed output of a fake `kiro-cli chat --list-models`.
// "auto" is Kiro's router and must never reach the cache.
const kiroListing = `{"models":[` +
	`{"model_id":"auto","model_name":"Auto"},` +
	`{"model_id":"claude-opus-5-5","model_name":"Opus"},` +
	`{"model_id":"claude-haiku-4-5","model_name":"Haiku"},` +
	`{"model_id":"claude-sonnet-5-5","model_name":"Sonnet"}]}`

// fakeKiroCLI writes a kiro-cli stand-in that prints listing for the exact
// listing command and exits 9 for anything else. It never runs the real CLI.
func fakeKiroCLI(t *testing.T, listing string, exit int) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "kiro-cli")
	script := "#!/bin/sh\n" +
		"[ \"$1\" = chat ] && [ \"$2\" = --list-models ] && [ \"$3\" = --format ] && [ \"$4\" = json ] || exit 9\n" +
		"cat <<'LISTING'\n" + listing + "\nLISTING\n" +
		"exit " + strconv.Itoa(exit) + "\n"
	if err := os.WriteFile(bin, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return bin
}

func TestDiscoverKiroCLIStoresSourceOrderAndRanksLeastToMost(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	src := discovery.KiroCLI{Bin: fakeKiroCLI(t, kiroListing, 0)}
	var out, errOut bytes.Buffer
	if code := discoverWith(catalog.Load(), []discovery.Source{src}, cache, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	entry := discovery.Read(cache).Harnesses["kirocrew"]
	var sourceOrder []string
	for _, m := range entry.Models {
		sourceOrder = append(sourceOrder, m.ID)
	}
	if got, want := strings.Join(sourceOrder, ","), "claude-opus-5-5,claude-haiku-4-5,claude-sonnet-5-5"; got != want {
		t.Fatalf("source order = %s, want %s (auto must be skipped)", got, want)
	}
	if got, want := strings.Join(entry.IDs, ","), "claude-haiku-4-5,claude-sonnet-5-5,claude-opus-5-5"; got != want {
		t.Fatalf("ordered = %s, want least to most capable %s", got, want)
	}
}

func TestDiscoverKiroCrewHarnessFilterRefreshesOnlyKiro(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	kiro := discovery.KiroCLI{Bin: fakeKiroCLI(t, kiroListing, 0)}
	codex := fakeSource{harness: "codex", models: []discovery.Model{{ID: "gpt-6-sol"}}}
	var out, errOut bytes.Buffer
	if code := discoverWith(catalog.Load(), []discovery.Source{kiro, codex}, cache, []string{"--harness=kirocrew"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	f := discovery.Read(cache)
	if _, ok := f.Harnesses["codex"]; ok || len(f.Harnesses["kirocrew"].IDs) != 3 {
		t.Fatalf("--harness=kirocrew must refresh only kirocrew: %+v", f.Harnesses)
	}
}

func TestDiscoverFailingKiroCLIKeepsPreviousEntry(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	var out, errOut bytes.Buffer
	good := discovery.KiroCLI{Bin: fakeKiroCLI(t, kiroListing, 0)}
	if code := discoverWith(catalog.Load(), []discovery.Source{good}, cache, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}

	// A failing kiro-cli and a healthy codex source in the same run: kiro keeps
	// its entry, and the healthy source is still written.
	failing := discovery.KiroCLI{Bin: fakeKiroCLI(t, "", 1)}
	codex := fakeSource{harness: "codex", models: []discovery.Model{{ID: "gpt-6-sol"}}}
	out.Reset()
	if code := discoverWith(catalog.Load(), []discovery.Source{failing, codex}, cache, nil, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	after := discovery.Read(cache)
	if got := strings.Join(after.Harnesses["kirocrew"].IDs, ","); got != "claude-haiku-4-5,claude-sonnet-5-5,claude-opus-5-5" {
		t.Fatalf("failed kiro-cli replaced the previous ordered list: %s", got)
	}
	if len(after.Harnesses["codex"].IDs) == 0 {
		t.Fatal("a failing source blocked a healthy one")
	}
	if !strings.Contains(out.String(), "kirocrew") || !strings.Contains(out.String(), "skipped") {
		t.Errorf("report must name the skipped kirocrew source:\n%s", out.String())
	}
}
