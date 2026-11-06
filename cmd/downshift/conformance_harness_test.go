// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// conformanceHarness is one harness under test: registry facts plus the
// catalog models of each tier.
type conformanceHarness struct {
	id                   string
	caps                 core.HarnessCapabilities
	driver               conformanceDriver
	small, mid, frontier core.Model
	eventLog             string
}

// run drives one spawn through the harness's registered hook (the shared
// runner) and decodes the response.
func (h conformanceHarness) run(t *testing.T, s spawnSpec) hookResult {
	t.Helper()
	if s.cwd == "" {
		s.cwd = t.TempDir()
	}
	if h.driver.chdir {
		t.Chdir(s.cwd)
	}
	hook := harnessRegistry[h.id].Hook
	var rc int
	var stderr string
	stdout := captureStdout(func() {
		stderr = captureStderr(func() { rc = hook(bytes.NewReader(h.driver.build(s)), cmdCat) })
	})
	return h.driver.decode(t, rc, stdout, stderr, s.model)
}

// session installs the session allowlist the hook will resolve for this harness.
func (h conformanceHarness) session(t *testing.T, models ...core.Model) {
	t.Helper()
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}
	setSessionAllowlist(t, h.id, ids)
}

// alternateSmall returns a second automatic small-tier catalog model of this
// harness, distinct from the default, that the harness can write. Zero when
// the catalog has none. Candidates are walked sorted by id: the catalog slice
// is shared in-process and another command may have reordered it, so the
// scenario's alternate must not depend on slice order.
func (h conformanceHarness) alternateSmall() core.Model {
	var candidates []core.Model
	for _, e := range cmdCat.Entries() {
		if e.Harness != h.id || e.Tier != core.TierSmall.String() || e.ID == h.small.ID || e.Routing == "explicit_only" {
			continue
		}
		if h.caps.StrictModelName && e.NativeName == "" {
			continue
		}
		if m, ok := cmdCat.LookupByID(h.id, e.ID); ok && m.ID == e.ID {
			candidates = append(candidates, m)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })
	if len(candidates) == 0 {
		return core.Model{}
	}
	return candidates[0]
}

// lastEvent returns the newest telemetry event the hook recorded.
func (h conformanceHarness) lastEvent(t *testing.T) telemetry.Event {
	t.Helper()
	events, err := telemetry.ReadEventsFrom(h.eventLog)
	if err != nil || len(events) == 0 {
		t.Fatalf("no telemetry event recorded (err=%v)", err)
	}
	return events[len(events)-1]
}

func writeProjectFloor(t *testing.T, dir, tier string) {
	t.Helper()
	p := filepath.Join(dir, ".downshift", "policy.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(`{"version":1,"min_tier":"`+tier+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}
