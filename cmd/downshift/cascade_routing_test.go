// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/cascade"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
)

func TestRouting_AutoUpgradeCascadeOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", tempDir)

	sessionID := "sess_cascade_test"
	defer hookctx.SetSessionID(sessionID)()

	prompt := "[tier=small] rename userId variable to userIdentifier"
	harness := "claude-code"
	frontier := cmdCat.ModelFor(harness, core.TierFrontier).ID
	small := cmdCat.ModelFor(harness, core.TierSmall).ID
	setSessionAllowlist(t, harness, []string{
		small,
		cmdCat.ModelFor(harness, core.TierMid).ID,
		frontier,
	})

	// 1. Initial run: should downshift to small tier for a simple rename task
	d1 := core.Route(prompt, harness, frontier, cmdCat)
	if d1.Verdict != core.VerdictDownshift {
		t.Fatalf("initial verdict = %s; want DOWNSHIFT for simple prompt", d1.Verdict)
	}

	// 2. Simulate sensor recording a test failure for this task
	cascade.RecordFailure(sessionID, prompt, d1.Model.ID, "go test failed with exit code 1")

	// 3. Second run: Auto-Upgrade Cascade should detect previous failure and upshift
	d2 := core.Route(prompt, harness, frontier, cmdCat)
	if d2.Verdict != core.VerdictUpshift {
		t.Fatalf("cascade verdict = %s; want UPSHIFT after recent failure", d2.Verdict)
	}
	if d2.Reason != "CASCADE_UPGRADE" {
		t.Errorf("expected Reason == CASCADE_UPGRADE, got %q", d2.Reason)
	}

	// 4. Clear failure: should return to normal downshift
	cascade.ClearSession(sessionID)
	d3 := core.Route(prompt, harness, frontier, cmdCat)
	if d3.Verdict != core.VerdictDownshift {
		t.Errorf("after clear, verdict = %s; want DOWNSHIFT", d3.Verdict)
	}
}
