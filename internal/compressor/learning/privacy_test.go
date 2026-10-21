// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// rawOutput is the kind of text the schema 1 store leaked: a branch name, a
// PR URL and file paths.
const rawOutput = "On branch feat/secret-branch-name\n" +
	"https://github.com/example/private-repo/pull/4242\n" +
	"  modified:   internal/secret/path/file.go\n" +
	"ok  \tgithub.com/x/internal/a\t0.1s\n" +
	"12345 67890 42\n"

var leakSubstrings = []string{"secret-branch-name", "feat/", "github.com", "pull/4242", "internal/secret", "file.go", "private-repo"}

func assertNoLeak(t *testing.T, label string, data []byte) {
	t.Helper()
	for _, s := range leakSubstrings {
		if strings.Contains(string(data), s) {
			t.Errorf("%s contains output text %q", label, s)
		}
	}
}

func TestRecord_StoresShapeNotText(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learning.json")
	SetStorePath(path)
	defer SetStorePath("")

	for i := 0; i < MinCountToSuggest; i++ {
		if err := Record([]string{"git", "status"}, []byte(rawOutput), execwrap.FamilyGitStatus); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertNoLeak(t, "store file", data)

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		t.Fatal(err)
	}
	if store.Version != SchemaVersion {
		t.Errorf("version = %d, want %d", store.Version, SchemaVersion)
	}
	var entry *Entry
	for _, e := range store.Entries {
		entry = e
	}
	if entry == nil || entry.Shape == nil {
		t.Fatal("entry with shape expected")
	}
	if entry.SampleBytes != len(rawOutput) {
		t.Errorf("sample_bytes = %d, want %d", entry.SampleBytes, len(rawOutput))
	}
	sh := entry.Shape
	if sh.Lines != 5 || sh.URLLike != 1 || sh.PathLike < 2 || sh.StatusLike != 1 || sh.NumberHeavy != 1 || sh.Indented != 1 {
		t.Errorf("unexpected shape: %+v", *sh)
	}
	if sh.FirstLineShape == "" {
		t.Error("first_line_shape expected")
	}

	// Suggestions and Export carry the same guarantee.
	sug, err := Suggestions(10)
	if err != nil {
		t.Fatal(err)
	}
	sb, _ := json.Marshal(sug)
	assertNoLeak(t, "suggestions", sb)
	st, err := Export()
	if err != nil {
		t.Fatal(err)
	}
	eb, _ := json.Marshal(st)
	assertNoLeak(t, "export", eb)
}

func TestLoad_MigratesSchema1DroppingSample(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learning.json")
	SetStorePath(path)
	defer SetStorePath("")

	old := `{"version":1,"entries":{"abc":{"signature":{"words":["git","status"],"hash":"abc"},` +
		`"count":7,"first_seen":"2026-10-01T00:00:00Z","last_seen":"2026-10-02T00:00:00Z",` +
		`"total_bytes":700,"sample_bytes":100,"sample":` + jsonString(rawOutput) + `}},"updated_at":"2026-10-02T00:00:00Z"}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	// A leftover of an interrupted schema 1 save.
	if err := os.WriteFile(path+".tmp", nil, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Export()
	if err != nil {
		t.Fatal(err)
	}
	if store.Version != SchemaVersion {
		t.Errorf("version = %d, want %d", store.Version, SchemaVersion)
	}
	e := store.Entries["abc"]
	if e == nil || e.Count != 7 || e.Shape != nil {
		t.Fatalf("entry not carried over: %+v", e)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertNoLeak(t, "migrated file", data)
	if !strings.Contains(string(data), `"version": 2`) {
		t.Error("file not rewritten at schema 2")
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("stale .tmp not removed")
	}
	if entries, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp")); len(entries) != 0 {
		t.Errorf("temp files left behind: %v", entries)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
