// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
)

// reportGoldenFixture seeds the shared report fixture with fixed timestamps, so
// `report --days=0` is byte-for-byte reproducible: the routing events of
// testdata/report_why plus one Compactor row.
func reportGoldenFixture(t *testing.T) {
	t.Helper()
	whyFixtureEnv(t)
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	ev := execwrap.Event{TS: "2026-10-01T12:00:00Z", Harness: "claude-code", RawBytes: 4400, EmittedBytes: 400, Compacted: true}
	if err := execwrap.Append(path, ev); err != nil {
		t.Fatal(err)
	}
}

// TestReportGolden pins `report --text` and `report --json` so a refactor of
// the report data can prove the bytes did not move. Regenerate with:
//
//	go test ./cmd/downshift -run TestReportGolden -update
func TestReportGolden(t *testing.T) {
	for _, flag := range []string{"--text", "--json"} {
		t.Run(flag, func(t *testing.T) {
			reportGoldenFixture(t)
			got := runReportOut(t, "--days=0", flag)
			path := filepath.Join("testdata", "report", "report"+flag[1:]+".golden")
			if *updateGoldens {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden %s (run with -update): %v", path, err)
			}
			if string(want) != got {
				t.Errorf("report %s changed\n--- want ---\n%s\n--- got ---\n%s", flag, want, got)
			}
		})
	}
}
