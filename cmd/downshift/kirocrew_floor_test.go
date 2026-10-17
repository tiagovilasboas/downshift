// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// The KiroCrew runner goes through the same project-context seam as the
// shared runner: a .downshift/policy.json floor in the payload's cwd keeps a
// trivial spawn at the frontier tier, so policy mode allows instead of blocks.
func TestRunKiroCrewHook_ProjectFloorApplies(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
	frontierID := cmdCat.ModelFor("kirocrew", core.TierFrontier).ID
	setSessionAllowlist(t, "kirocrew", []string{
		cmdCat.ModelFor("kirocrew", core.TierSmall).ID,
		cmdCat.ModelFor("kirocrew", core.TierMid).ID,
		frontierID,
	})
	project := t.TempDir()
	if err := os.MkdirAll(filepath.Join(project, ".downshift"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".downshift", "policy.json"), []byte(`{"version":1,"min_tier":"frontier"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := func(cwd string) string {
		return `{"tool_name":"spawn_run","cwd":"` + cwd + `","tool_input":{"task":"rename the userId variable to userIdentifier","model":"` + frontierID + `"}}`
	}
	var rc int
	captureStderr(func() { rc = runKiroCrewHook(strings.NewReader(payload(project)), cmdCat) })
	if rc != 0 {
		t.Fatalf("with a frontier floor the spawn must be allowed, rc=%d", rc)
	}
	captureStderr(func() { rc = runKiroCrewHook(strings.NewReader(payload(t.TempDir())), cmdCat) })
	if rc != 2 {
		t.Fatalf("without a floor the trivial spawn is blocked toward a smaller tier, rc=%d", rc)
	}
}
