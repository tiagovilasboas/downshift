// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/testenv"
)

// periodsState writes an exec log with two harnesses on two recent days.
func periodsState(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	day := func(back int) string {
		return time.Now().UTC().Add(-time.Duration(back) * 24 * time.Hour).Format(time.RFC3339)
	}
	body := `{"ts":"` + day(1) + `","harness":"claude-code","raw_bytes":4000,"emitted_bytes":0,"compacted":true}` + "\n" +
		`{"ts":"` + day(1) + `","harness":"codex","raw_bytes":800,"emitted_bytes":0,"compacted":true}` + "\n" +
		`{"ts":"` + day(2) + `","harness":"claude-code","raw_bytes":400,"emitted_bytes":0,"compacted":true}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "exec-events.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReportDailyJSON(t *testing.T) {
	periodsState(t)
	var out, errOut bytes.Buffer
	if code := runReport([]string{"--daily", "--days=5", "--json"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	var got periodsOut
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("not json: %v\n%s", err, out.String())
	}
	if got.Per != "day" || got.Days != 5 || len(got.Rows) != 5 {
		t.Fatalf("shape = per %q days %d rows %d", got.Per, got.Days, len(got.Rows))
	}
	if strings.Join(got.Harnesses, ",") != "claude-code,codex" {
		t.Errorf("harnesses = %v", got.Harnesses)
	}
	var total int64
	for _, r := range got.Rows {
		for _, h := range r.Harnesses {
			total += h.CompactionTokens
		}
	}
	if total != 1000+200+100 {
		t.Errorf("compaction tokens = %d, want 1300", total)
	}
	for _, bad := range []string{"usd", "cost", "dollar", "$"} {
		if strings.Contains(out.String(), bad) {
			t.Errorf("output mentions %q", bad)
		}
	}
}

func TestReportDailyAndHistoryText(t *testing.T) {
	periodsState(t)
	for _, args := range [][]string{{"--daily", "--days=3"}, {"--history"}, {"--history", "--by=day"}} {
		var out, errOut bytes.Buffer
		if code := runReport(args, &out, &errOut); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, errOut.String())
		}
		if !strings.Contains(out.String(), "claude-code saved") || !strings.Contains(out.String(), "codex below") {
			t.Errorf("%v: missing harness columns:\n%s", args, out.String())
		}
	}
}

func TestReportPeriodsEmptyWindow(t *testing.T) {
	testenv.SetHome(t, t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	var out, errOut bytes.Buffer
	if code := runReport([]string{"--history"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "No compaction or routed spawns") {
		t.Errorf("empty output = %q", out.String())
	}
}

func TestResolvePeriodsFlags(t *testing.T) {
	tests := []struct {
		name          string
		daily, hist   bool
		by            string
		days          int
		daysSet       bool
		wantErr       bool
		wantPer       string
		wantHistoryOn bool
	}{
		{"daily default window", true, false, "", 30, false, false, "day", false},
		{"history defaults to weeks", false, true, "", 30, false, false, "week", true},
		{"history by day", false, true, "day", 30, false, false, "day", true},
		{"both", true, true, "", 30, false, true, "", false},
		{"bad by", false, true, "month", 30, false, true, "", false},
		{"by without history", true, false, "day", 30, false, true, "", false},
		{"history with days", false, true, "", 7, true, true, "", false},
		{"daily zero days", true, false, "", 0, true, true, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, msg := resolvePeriods(tc.daily, tc.hist, tc.by, tc.days, tc.daysSet)
			if (msg != "") != tc.wantErr {
				t.Fatalf("msg = %q, wantErr %v", msg, tc.wantErr)
			}
			if !tc.wantErr && (req.per != tc.wantPer || req.history != tc.wantHistoryOn) {
				t.Errorf("req = %+v", req)
			}
		})
	}
}

func TestReportWithoutPeriodFlagsIsUnchanged(t *testing.T) {
	periodsState(t)
	var out, errOut bytes.Buffer
	if code := runReport([]string{"--json"}, &out, &errOut); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut.String())
	}
	if strings.Contains(out.String(), `"per"`) || !strings.Contains(out.String(), `"numbers"`) {
		t.Errorf("default report changed:\n%.300s", out.String())
	}
}
