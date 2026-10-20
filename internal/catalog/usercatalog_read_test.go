// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestReadUserCatalogRejectsOversizedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	big := make([]byte, maxUserCatalogBytes+1)
	if err := os.WriteFile(path, big, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readUserCatalog(path); err == nil {
		t.Fatal("a catalog over the cap must be rejected")
	}
}

func TestReadUserCatalogRejectsNonRegularFile(t *testing.T) {
	if _, err := readUserCatalog(t.TempDir()); err == nil {
		t.Fatal("a directory must be rejected")
	}
	if _, err := readUserCatalog(os.DevNull); err == nil {
		t.Fatal("a device must be rejected")
	}
}

func TestReadUserCatalogReadsRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readUserCatalog(path)
	if err != nil || string(got) != "{}" {
		t.Fatalf("got %q, %v", got, err)
	}
}

// The override is untrusted input read on every hook: whatever it holds, load
// returns the embedded entries and never panics.
func TestLoadSurvivesHostileOverrides(t *testing.T) {
	// Count the embedded entries under an empty HOME: a real ~/.downshift
	// (user overrides, openrouter.json) would otherwise change the baseline.
	t.Setenv("HOME", t.TempDir())
	embedded := len(load(&strings.Builder{}).Entries())
	for name, body := range map[string]string{
		"null":           `null`,
		"array":          `[]`,
		"null entry":     `{"entries":[null]}`,
		"empty entry":    `{"entries":[{}]}`,
		"wrong types":    `{"entries":"x"}`,
		"deep nesting":   strings.Repeat("[", 5000),
		"binary":         "\x00\xff\xfe",
		"not json":       `{"entries":[`,
		"huge tier name": `{"entries":[{"id":"x","harness":"claude-code","tier":"` + strings.Repeat("a", 1<<16) + `"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			p, err := paths.CatalogPath()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			c := load(&strings.Builder{})
			if c == nil || len(c.Entries()) < embedded {
				t.Fatalf("lost the embedded entries: %v", c)
			}
		})
	}
}
