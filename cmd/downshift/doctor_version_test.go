// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/setup"
	"github.com/tiagovilasboas/downshift-ai/internal/setup/configfile"
)

// writePluginStamp registers a user install of the Claude Code plugin at version.
func writePluginStamp(t *testing.T, home, version string) {
	t.Helper()
	reg := filepath.Join(home, ".claude", filepath.FromSlash(setup.PluginRegistryRel))
	if err := os.MkdirAll(filepath.Dir(reg), 0o700); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"plugins": map[string]any{"downshift@downshift": []map[string]string{
		{"scope": "user", "installPath": filepath.Join(home, "cache"), "version": version}}}}
	raw, _ := json.Marshal(doc)
	if err := os.WriteFile(reg, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func versionRow(rep *doctorReport, id string) (doctorCheck, bool) {
	for _, c := range rep.Checks {
		if c.ID == id {
			return c, true
		}
	}
	return doctorCheck{}, false
}

func TestCheckVersionSyncPluginStamp(t *testing.T) {
	cases := []struct {
		name, binary, stamp, wantStatus string
	}{
		{"match", "v0.8.1", "0.8.1", doctorOK},
		{"mismatch fails", "v0.8.1", "0.7.0", doctorFail},
		{"commit stamp is not a version", "v0.8.1", "9f2c1ab4d5e6", ""},
		{"dev binary is not compared", "dev", "0.7.0", ""},
		{"no plugin installed", "v0.8.1", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			if c.stamp != "" {
				writePluginStamp(t, home, c.stamp)
			}
			rep := &doctorReport{}
			checkVersionSync(rep, doctorEnv{Home: home}, c.binary)
			row, ok := versionRow(rep, "version sync")
			if c.wantStatus == "" {
				if ok {
					t.Fatalf("expected no row, got %+v", row)
				}
				return
			}
			if !ok || row.Status != c.wantStatus {
				t.Fatalf("got %+v ok=%v, want %s", row, ok, c.wantStatus)
			}
			if c.wantStatus == doctorFail && !strings.Contains(row.Fix, "downshift setup") {
				t.Errorf("fix must name downshift setup: %q", row.Fix)
			}
			if c.wantStatus == doctorFail && rep.Failed != 1 {
				t.Errorf("Failed = %d", rep.Failed)
			}
		})
	}
}

func TestCheckVersionSyncStaleBridge(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "opencode", "plugins", "downshift")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stale := configfile.OwnedPhrase + " old bridge\n"
	for _, f := range setup.OpenCodeBridgeFiles() {
		if err := os.WriteFile(filepath.Join(home, filepath.FromSlash(f.Dest)), []byte(stale), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rep := &doctorReport{}
	checkVersionSync(rep, doctorEnv{Home: home}, "v0.8.1")
	row, ok := versionRow(rep, "version sync")
	if !ok || row.Status != doctorFail || !strings.Contains(row.Fix, "downshift setup") {
		t.Fatalf("got %+v ok=%v", row, ok)
	}
}

func TestCheckVersionSyncPathNewest(t *testing.T) {
	// Absolute on every OS: doctor runs only absolute PATH entries.
	root := t.TempDir()
	a, b := filepath.Join(root, "a", "downshift"), filepath.Join(root, "b", "downshift")
	versions := map[string]string{a: "v0.7.0", b: "v0.8.1"}
	run := func(p string) (string, error) { return "downshift " + versions[p], nil }
	paths := func(copies ...string) func() []string { return func() []string { return copies } }
	cases := []struct {
		name   string
		copies []string
		want   string
	}{
		{"first is older", []string{a, b}, doctorWarn},
		{"first is newest", []string{b, a}, doctorOK},
		{"single copy", []string{a}, ""},
		{"relative entry is not run", []string{filepath.Join("bin", "downshift"), a}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := &doctorReport{}
			env := doctorEnv{Home: t.TempDir(), RunVersion: run, PathCopies: paths(c.copies...)}
			checkVersionSync(rep, env, "v0.8.1")
			row, ok := versionRow(rep, "newest on PATH")
			if c.want == "" {
				if ok {
					t.Fatalf("unexpected row %+v", row)
				}
				return
			}
			if !ok || row.Status != c.want {
				t.Fatalf("got %+v ok=%v want %s", row, ok, c.want)
			}
			if c.want == doctorWarn && !strings.Contains(row.Detail, b) {
				t.Errorf("detail must name the newest copy: %q", row.Detail)
			}
		})
	}
}

func TestCheckVersionSyncLatest(t *testing.T) {
	cases := []struct {
		name, binary, tag string
		known             bool
		want              string
		wantFailed        int
	}{
		{"newer release", "v0.7.0", "v0.8.1", true, doctorWarn, 0},
		{"up to date", "v0.8.1", "v0.8.1", true, doctorOK, 0},
		{"offline is unknown, not a failure", "v0.7.0", "", false, doctorOK, 0},
		{"dev binary", "dev", "v0.8.1", true, doctorOK, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rep := &doctorReport{}
			env := doctorEnv{Home: t.TempDir(), Latest: func() (string, bool) { return c.tag, c.known }}
			checkVersionSync(rep, env, c.binary)
			row, ok := versionRow(rep, "latest release")
			if !ok || row.Status != c.want || rep.Failed != c.wantFailed {
				t.Fatalf("got %+v ok=%v failed=%d", row, ok, rep.Failed)
			}
			if !c.known && !strings.Contains(row.Detail, "unknown") {
				t.Errorf("offline detail must say unknown: %q", row.Detail)
			}
		})
	}
}

func TestCheckVersionSyncDefaultIsOffline(t *testing.T) {
	rep := &doctorReport{}
	checkVersionSync(rep, doctorEnv{Home: t.TempDir()}, "v0.8.1")
	if _, ok := versionRow(rep, "latest release"); ok {
		t.Fatal("latest release must only be checked on request")
	}
}
