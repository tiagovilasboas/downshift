// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/discovery"
	"github.com/tiagovilasboas/downshift/internal/setup"
)

// setupEnsureSessionModels is a read-only step: it never writes
// session-models.json. The hook already reads the live discovery cache before
// that file (docs/reference/session-models.md), so copying the detected list
// into it would only make a copy that goes stale. The file stays an optional
// manual override the operator writes.
//
// For each target whose models come from a list (not the catalog stand-in) and
// that has neither a fresh discovery entry nor a file list, it prints the
// command that detects the list. A harness with a fresh entry prints nothing.
func setupEnsureSessionModels(o setup.Options, targets []setup.Target) {
	if o.DryRun || o.Undo {
		return
	}
	now := time.Now()
	for _, target := range targets {
		h := target.Name
		if caps, known := core.CapsFor(h); !known || caps.SessionFromCatalog {
			continue
		}
		if core.LoadDiscoveredSession(h, now).Known || core.LoadUserSession(h).Known {
			continue
		}
		fmt.Fprintf(o.Out, "models: no detected list for %s; run `downshift models discover --harness %s`\n", h, h)
	}
}

// setupDiscoverModels runs models discovery for harnesses with a source
// so fresh installs don't stay on handwritten or outdated lists.
func setupDiscoverModels(o setup.Options) {
	if o.SkipDiscovery || o.DryRun || o.Undo {
		return
	}
	cachePath, err := discovery.Path()
	if err != nil {
		return
	}
	cat := catalog.Load()
	sources := discovery.Defaults()
	now := o.Now().UTC()
	outcomes := discovery.Run(context.Background(), sources, 5*time.Second)
	updates := map[string]discovery.Entry{}
	for _, res := range outcomes {
		if res.Err != nil || len(res.Models) == 0 {
			continue
		}
		h := res.Source.Harness()
		ranked := discovery.Order(h, res.Models, cat)
		updates[h] = discovery.Entry{
			Source:    res.Source.Name(),
			FetchedAt: now,
			Models:    res.Models,
			Ranked:    ranked,
		}
	}
	if len(updates) > 0 {
		_ = discovery.Merge(cachePath, updates)
	}
}
