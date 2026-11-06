// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func (f *doctorFixture) writeDecision(t *testing.T, age time.Duration, verdict, outcome, from, to string) {
	t.Helper()
	ts := f.started.Add(-age).Format(time.RFC3339Nano)
	line := fmt.Sprintf(`{"correlation_id":"d1","timestamp":%q,"harness":"claude-code","requested_model":%q,"final_model":%q,"verdict":%q,"outcome":%q}`, ts, from, to, verdict, outcome)
	writeFile(t, filepath.Join(f.home, ".downshift", "events.jsonl"), line+"\n")
}

func TestDoctor_LastEventNamesTheDecision(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))

	f.writeDecision(t, 5*time.Minute, "DOWNSHIFT", "rewrite_emitted", "opus", "haiku")
	c := f.check(t, buildDoctorReport(f.env), "claude-code", "last hook event")
	wantStatus(t, c, doctorOK)
	for _, want := range []string{"5m ago", "DOWNSHIFT", "rewrite_emitted", "opus -> haiku"} {
		if !strings.Contains(c.Detail, want) {
			t.Errorf("detail %q misses %q", c.Detail, want)
		}
	}

	// A held spawn keeps the model, so no arrow is shown.
	f.writeDecision(t, time.Minute, "OK", "allow", "haiku", "haiku")
	c = f.check(t, buildDoctorReport(f.env), "claude-code", "last hook event")
	if strings.Contains(c.Detail, "->") || !strings.Contains(c.Detail, "allow") {
		t.Errorf("an unchanged model must not show an arrow: %q", c.Detail)
	}
}

func TestDoctor_LastEventIgnoresUsageRowsAsDecision(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
	f.writeDecision(t, time.Minute, "OK", "usage", "haiku", "haiku")
	c := f.check(t, buildDoctorReport(f.env), "claude-code", "last hook event")
	if strings.Contains(c.Detail, "last decision") {
		t.Errorf("a usage row is not a decision: %q", c.Detail)
	}
}

func TestDoctor_QuotaSourceStaleness(t *testing.T) {
	for _, tc := range []struct {
		name       string
		expiredAgo time.Duration // storeQuota observes one minute before expiry
		want       string
		wantFix    bool
	}{
		{"fresh", -time.Minute, doctorOK, false},
		{"exactly at the limit", 14 * time.Minute, doctorOK, false},
		{"just past the limit", 15 * time.Minute, doctorWarn, true},
		{"nine days", 9 * 24 * time.Hour, doctorWarn, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newDoctorFixture(t)
			writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
			f.storeQuota(t, tc.expiredAgo)
			c := f.check(t, buildDoctorReport(f.env), "claude-code", "quota source")
			wantStatus(t, c, tc.want)
			if tc.wantFix && (!strings.Contains(c.Detail, "freshness limit") || !strings.Contains(c.Detail, "15m") || !strings.Contains(c.Fix, "interactive terminal") || !strings.Contains(c.Fix, "claude -p")) {
				t.Errorf("stale detail/fix should state age, limit and why the statusline never refreshed: %q / %q", c.Detail, c.Fix)
			}
		})
	}
}

func TestDoctor_QuotaSource(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "none.json"))

	c := f.check(t, buildDoctorReport(f.env), "claude-code", "quota source")
	wantStatus(t, c, doctorOK)
	if !strings.Contains(c.Detail, "none yet") {
		t.Errorf("detail = %q", c.Detail)
	}

	f.storeQuota(t, -time.Hour) // still valid for another hour
	c = f.check(t, buildDoctorReport(f.env), "claude-code", "quota source")
	wantStatus(t, c, doctorOK)
	if !strings.Contains(c.Detail, "claude-statusline") {
		t.Errorf("the source should be named: %q", c.Detail)
	}
}
