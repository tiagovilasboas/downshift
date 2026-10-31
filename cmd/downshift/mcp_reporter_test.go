// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
)

func TestMcpReporterStoresReportedIDsInTheDiscoveryCache(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "discovered.json")
	t.Setenv(discovery.EnvPath, cache)
	cat := catalog.Load()
	h := core.AllHarnessCaps()[0].Harness
	id := cat.ModelFor(h, core.TierSmall).ID
	now := time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)

	r := mcpReporter{cat: cat, now: func() time.Time { return now }}
	out, err := r.Report(context.Background(), h, []string{id})
	if err != nil || !out.Written || out.Accepted != 1 {
		t.Fatalf("outcome %+v err %v", out, err)
	}
	// h is SessionFromCatalog (claude-code): the reported id is added to the
	// catalog stand-in the router already saw, so a one-id report never
	// narrows the list. The stand-in has 4 models; id is one of them.
	e := discovery.Read(cache).Harnesses[h]
	if e.Source != "mcp" || len(e.Models) != 4 {
		t.Fatalf("entry %+v", e)
	}
	found := false
	for _, m := range e.Models {
		if m.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatalf("reported id %q missing from the stored union: %+v", id, e.Models)
	}
}

func TestMcpReporterHonorsACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := mcpReporter{cat: catalog.Load(), now: time.Now}.Report(ctx, "x", []string{"a"})
	if err == nil {
		t.Fatal("a cancelled context must fail")
	}
}
