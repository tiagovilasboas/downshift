// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"os"
	"path/filepath"
	"testing"
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
