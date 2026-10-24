// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package harness2export_test

import (
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/harness2export"
)

func TestMatrix_ClaudeCodeProductionEligible(t *testing.T) {
	hook := func(id string) bool { return id == "claude-code" }
	capsFor := func(id string) (harness2export.CapsSnapshot, bool) {
		c, ok := core.CapsFor(id)
		if !ok {
			return harness2export.CapsSnapshot{}, false
		}
		return harness2export.CapsSnapshot{
			CanRewriteModel:        c.CanRewriteModel,
			CanApplyEffort:         c.CanApplyEffort,
			SingleModelReasoning:   c.SingleModelReasoning,
			CanRewriteShellCommand: c.CanRewriteShellCommand,
		}, true
	}
	rows := harness2export.Matrix(hook, capsFor)
	var cc *harness2export.Row
	for i := range rows {
		if rows[i].ID == "claude-code" {
			cc = &rows[i]
			break
		}
	}
	if cc == nil {
		t.Fatal("missing claude-code row")
	}
	if cc.Maturity != harness2export.MaturityProduction || cc.Role != harness2export.RoleMutate {
		t.Fatalf("maturity=%q role=%q", cc.Maturity, cc.Role)
	}
	if !cc.RecursiveHarness2Eligible || !cc.HasHook {
		t.Fatalf("eligible=%v hook=%v", cc.RecursiveHarness2Eligible, cc.HasHook)
	}
}

func TestMatrix_GrokConfigOnlyNoHook(t *testing.T) {
	hook := func(string) bool { return false }
	capsFor := func(id string) (harness2export.CapsSnapshot, bool) {
		if id != "grok" {
			return harness2export.CapsSnapshot{}, false
		}
		return harness2export.CapsSnapshot{SingleModelReasoning: true}, true
	}
	for _, row := range harness2export.Matrix(hook, capsFor) {
		if row.ID != "grok" {
			continue
		}
		if row.HasHook || row.Maturity != harness2export.MaturityConfigOnly {
			t.Fatalf("grok: hook=%v maturity=%q", row.HasHook, row.Maturity)
		}
		if row.Role != harness2export.RoleClassifierOnly || row.RecursiveHarness2Eligible {
			t.Fatalf("grok role=%q eligible=%v", row.Role, row.RecursiveHarness2Eligible)
		}
		return
	}
	t.Fatal("missing grok row")
}
