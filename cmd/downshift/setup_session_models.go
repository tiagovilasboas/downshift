// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
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
func setupEnsureSessionModels(o setupOptions, targets []setupTarget) {
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
