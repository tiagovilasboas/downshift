// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package harness2export_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/harness2export"
)

func TestMaturityMatchesHarnessMatrix(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "reference", "harness-matrix.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read matrix: %v", err)
	}
	documented := parseStatusTable(string(body))
	if len(documented) == 0 {
		t.Fatal("status table not found in harness-matrix.md")
	}
	hook := func(id string) bool { return id != "grok" }
	rows := harness2export.Matrix(hook, func(string) (harness2export.CapsSnapshot, bool) {
		return harness2export.CapsSnapshot{}, true
	})
	if len(rows) != len(documented) {
		t.Fatalf("matrix rows=%d documented harnesses=%d", len(rows), len(documented))
	}
	byID := map[string]harness2export.Row{}
	for _, row := range rows {
		byID[row.ID] = row
	}
	for id, status := range documented {
		row, ok := byID[id]
		if !ok {
			t.Errorf("harness-matrix.md lists %q (%s) missing from harness2export", id, status)
			continue
		}
		want, ok := harness2export.MaturityOf(status)
		if !ok {
			t.Errorf("unknown status %q for %s", status, id)
			continue
		}
		if row.Maturity != want {
			t.Errorf("%s maturity=%q, harness-matrix.md status %q maps to %q", id, row.Maturity, status, want)
		}
		if row.Role != harness2export.RoleOf(want) {
			t.Errorf("%s role=%q, want %q", id, row.Role, harness2export.RoleOf(want))
		}
		eligible := row.Role == harness2export.RoleMutate && row.HasHook
		if row.RecursiveHarness2Eligible != eligible {
			t.Errorf("%s eligible=%v, want %v", id, row.RecursiveHarness2Eligible, eligible)
		}
	}
	if err := harness2export.Judge(rows); err != nil {
		t.Fatal(err)
	}
}

func parseStatusTable(md string) map[string]string {
	out := map[string]string{}
	in := false
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "| harness |") {
			in = true
			continue
		}
		if !in {
			continue
		}
		if !strings.HasPrefix(line, "|") {
			break
		}
		if strings.HasPrefix(line, "|---") {
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 3 {
			continue
		}
		id := strings.TrimSpace(cols[1])
		status := strings.TrimSpace(cols[2])
		if id == "" || id == "harness" {
			continue
		}
		out[id] = status
	}
	return out
}
