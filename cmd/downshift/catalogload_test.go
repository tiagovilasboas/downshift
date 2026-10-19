// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

func TestLoadCatalogFailOpenRecoversPanic(t *testing.T) {
	old := catalogLoader
	t.Cleanup(func() { catalogLoader = old })
	catalogLoader = func() *catalog.Catalog { panic("boom") }
	var w bytes.Buffer
	c, ok := loadCatalogFailOpen(&w)
	if ok || c != nil {
		t.Fatalf("got ok=%v catalog=%v; want a recovered failure", ok, c)
	}
	if !strings.Contains(w.String(), "spawn allowed unchanged") {
		t.Errorf("no fail-open line on the writer: %q", w.String())
	}
}

func TestLoadCatalogFailOpenLoadsNormally(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	c, ok := loadCatalogFailOpen(&bytes.Buffer{})
	if !ok || c == nil {
		t.Fatalf("normal load failed: ok=%v", ok)
	}
}
