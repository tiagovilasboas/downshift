// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// The class step of the production decider (routing/decide classChoice) moves
// a spawn down only for a decisive TRIVIAL or SIMPLE task with no risk
// category. With no declared rule, profile or hint, a task that touches a risk
// category or a COMPLEX task on the frontier model is never moved down, for
// each adapter payload through the shared runner. A declared rule or the
// orchestrator's hint may still move them (owner decision O8); routing/decide
// TestDecide covers that. The control proves the class step is live.
func TestHookE2E_ClassStepNeverMovesRiskOrComplexDown(t *testing.T) {
	riskTasks := []string{
		"rename the apiKey variable in the secrets loader and rotate the stored token",
		"rename the userId field in the auth login handler that checks the password",
		"rename the amount variable in the payment refund handler for credit card charges",
		"rename the flag in the script that runs rm -rf on the production data directory",
	}
	complexTask := "design and implement a distributed consensus protocol with leader election, " +
		"log replication and snapshotting across services, then refactor the storage engine for concurrent " +
		"transactions with full test coverage"
	for _, task := range riskTasks {
		if len(core.RiskCategories(task)) == 0 {
			t.Fatalf("precondition: %q must touch a risk category", task)
		}
	}
	if c := core.ClassifyWithSemantic(complexTask); c.Complexity != core.Complex {
		t.Fatalf("precondition: complex task classified %v", c.Complexity)
	}
	for _, a := range doubtAdapters() {
		t.Run(a.harness, func(t *testing.T) {
			testenv.SetHome(t, t.TempDir())
			t.Setenv("DOWNSHIFT_EVENT_LOG", filepath.Join(t.TempDir(), "events.jsonl"))
			small := cmdCat.ModelFor(a.harness, core.TierSmall)
			mid := cmdCat.ModelFor(a.harness, core.TierMid)
			frontier := cmdCat.ModelFor(a.harness, core.TierFrontier)
			setSessionAllowlist(t, a.harness, []string{small.ID, mid.ID, frontier.ID})
			cwd := t.TempDir()
			hook := func(task string) string {
				stdin, _ := json.Marshal(a.payload(cwd, frontier.ID, task))
				return a.run(stdin)
			}
			movedDown := func(out string) bool {
				for _, m := range []core.Model{small, mid} {
					if strings.Contains(out, `"`+m.ID+`"`) || strings.Contains(out, `"`+m.WriteName()+`"`) {
						return true
					}
				}
				return false
			}
			for _, task := range append(riskTasks, complexTask) {
				if out := hook(task); movedDown(out) {
					t.Errorf("%q on %s was moved down: %s", task, frontier.ID, out)
				}
			}
			if out := hook(doubtTask); !movedDown(out) {
				t.Errorf("control: a decisive TRIVIAL task must move down, hook wrote %s", out)
			}
		})
	}
}
