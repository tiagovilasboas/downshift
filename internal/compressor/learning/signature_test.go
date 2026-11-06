// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var privateStrings = []string{
	"user", "tok", "host", "https", "scripts", "deploy", "prod",
	"foo", "bar", "=", "origin", "feat", "/", "@", "secret",
}

func TestRecord_StoresNoPathCredentialPairOrBranch(t *testing.T) {
	cases := []struct {
		name  string
		argv  []string
		words []string
	}{
		{"url with credentials", []string{"curl", "https://user:tok@host/x"}, []string{"curl"}},
		{"relative script", []string{"./scripts/deploy.sh", "prod"}, []string{"other"}},
		{"env assignment", []string{"FOO=bar", "go", "test"}, []string{"other"}},
		{"branch name", []string{"git", "push", "origin", "feat/x"}, []string{"git", "push"}},
		{"absolute path command", []string{"/usr/local/bin/Secret.tool", "run"}, []string{"other"}},
		{"windows path command", []string{`C:\tools\go.exe`, "test"}, []string{"go", "test"}},
		{"long token", []string{"tool", strings.Repeat("a", 40)}, []string{"tool"}},
		{"flag", []string{"go", "-v"}, []string{"go"}},
		{"grep pattern", []string{"grep", "hunter2", "."}, []string{"grep"}},
		{"rg key id", []string{"rg", "AKIAABCDEFGH12345678"}, []string{"rg"}},
		{"ls dir name", []string{"ls", "acme-prod-secrets"}, []string{"ls"}},
		{"uppercase subcommand", []string{"git", "AKIAABCDEFGH"}, []string{"git"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "learning.json")
			SetStorePath(path)
			defer SetStorePath("")
			if err := Record(tc.argv, []byte("out"), ""); err != nil {
				t.Fatal(err)
			}
			got := signatureOf(tc.argv).Words
			if strings.Join(got, " ") != strings.Join(tc.words, " ") {
				t.Errorf("words = %v, want %v", got, tc.words)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range privateStrings {
				if strings.Contains(strings.ToLower(string(data)), s) && !allowedBy(tc.words, s) {
					t.Errorf("store contains %q: %s", s, data)
				}
			}
		})
	}
}

// allowedBy reports whether s is a substring of a word the test expects.
func allowedBy(words []string, s string) bool {
	return strings.Contains(strings.Join(words, " "), s)
}

func TestLoad_CleansLegacyKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "learning.json")
	SetStorePath(path)
	defer SetStorePath("")
	if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}

	legacy := `{"version":2,"entries":{` +
		`"a":{"signature":{"words":["curl","https://user:tok@host/x"],"hash":"a"},"count":2,"total_bytes":10},` +
		`"b":{"signature":{"words":["./scripts/deploy.sh","prod"],"hash":"b"},"count":3,"total_bytes":5},` +
		`"c":{"signature":{"words":["foo=bar","go","test"],"hash":"c"},"count":1,"total_bytes":1},` +
		`"d":{"signature":{"words":["git","push","origin"],"hash":"d"},"count":4,"total_bytes":8},` +
		`"e":{"signature":{"words":["git","push","feat/x"],"hash":"e"},"count":6,"total_bytes":9}` +
		`},"updated_at":"2026-10-02T00:00:00Z"}`
	if err := os.WriteFile(path, []byte(legacy), 0o640); err != nil {
		t.Fatal(err)
	}

	store, err := Export()
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Entries) != 2 {
		t.Fatalf("entries = %d, want 2 (curl, git push merged): %+v", len(store.Entries), store.Entries)
	}
	var push *Entry
	for _, e := range store.Entries {
		if strings.Join(e.Signature.Words, " ") == "git push" {
			push = e
		}
	}
	if push == nil || push.Count != 10 || push.TotalBytes != 17 {
		t.Fatalf("git push entries not merged: %+v", push)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"user", "tok", "host", "scripts", "deploy", "foo", "origin", "feat", "/", "="} {
		if strings.Contains(string(data), s) {
			t.Errorf("rewritten file still contains %q: %s", s, data)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600 (never wider)", info.Mode().Perm())
	}
	if m, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*.tmp")); len(m) != 0 {
		t.Errorf("temp files left: %v", m)
	}
}
