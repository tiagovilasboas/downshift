// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func TestReport_MainSessionWorkRow(t *testing.T) {
	belowFixtureEnv(t)
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	raw, err := os.ReadFile(filepath.Join("testdata", "report_main_work", "main-work.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main-work.jsonl"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	sec := sectionOf(t, runReportOut(t, "--days=0", "--text"), "MAIN SESSION MECHANICAL WORK")
	sec = sec[:strings.Index(sec, "QUOTA USED PER HARNESS")]
	for _, h := range core.AllHarnessCaps() {
		var line string
		for _, l := range strings.Split(sec, "\n") {
			if strings.HasPrefix(strings.TrimSpace(l), h.Harness+" ") {
				line = l
			}
		}
		switch {
		case line == "":
			t.Errorf("no row for %s:\n%s", h.Harness, sec)
		case h.Caps.MainWorkObserved() && !strings.Contains(line, "main session: 3 read/search/shell calls, 2 on a frontier model"):
			t.Errorf("%s row = %q", h.Harness, line)
		case !h.Caps.MainWorkObserved() && !strings.Contains(line, "unobserved"):
			t.Errorf("%s must read unobserved: %q", h.Harness, line)
		}
	}
}
