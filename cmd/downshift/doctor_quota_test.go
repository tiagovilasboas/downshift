// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/quota"
)

func TestCheckQuotaSource_Desktop(t *testing.T) {
	cases := []struct {
		name    string
		desktop bool
		expired time.Duration // 0 = no snapshot
		want    string
		fix     string
	}{
		{"desktop no snapshot", true, 0, doctorWarn, claudeStatuslineFix},
		{"terminal no snapshot", false, 0, doctorOK, ""},
		{"fresh", true, -time.Minute, doctorOK, ""},
		{"stale", false, 2 * time.Hour, doctorWarn, claudeStatuslineFix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newDoctorFixture(t)
			home := f.home
			t.Setenv("DOWNSHIFT_STATE_DIR", filepath.Join(home, "state"))
			if runtime.GOOS == "linux" {
				t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
			}
			dir := claudeConfigDir(t, home)
			if dir == "" {
				t.Skipf("no known Claude config dir for GOOS %q", runtime.GOOS)
			}
			if tc.desktop {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if tc.expired != 0 {
				f.storeQuota(t, tc.expired)
			} else {
				t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
			}
			var status, fix string
			checkQuotaSource(func(_, s, _, x string) { status, fix = s, x }, f.env, "claude-code")
			if status != tc.want || fix != tc.fix {
				t.Errorf("status %q fix %q; want %q %q", status, fix, tc.want, tc.fix)
			}
		})
	}
}

// storeQuota writes a valid claude-code snapshot that expired expiredAgo before
// the fixture clock.
func (f *doctorFixture) storeQuota(t *testing.T, expiredAgo time.Duration) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "quota.json")
	t.Setenv("DOWNSHIFT_QUOTA_FILE", path)
	expires := f.started.Add(-expiredAgo)
	used := 10.0
	err := quota.Store(path, quota.Snapshot{Version: 1, Harness: "claude-code", Source: "claude-statusline",
		ObservedAt: expires.Add(-time.Minute), ExpiresAt: expires,
		Windows: []quota.Window{{ID: "5h", Scope: "harness", UsedPercent: &used, ResetsAt: f.started.Add(24 * time.Hour)}}})
	if err != nil {
		t.Fatal(err)
	}
}

func hasCheck(rep *doctorReport, harness, id string) bool {
	for _, c := range rep.Checks {
		if c.Harness == harness && c.ID == id {
			return true
		}
	}
	return false
}

func TestDoctor_QuotaSnapshotHolding(t *testing.T) {
	f := newDoctorFixture(t)
	writeFile(t, filepath.Join(f.home, ".claude", "settings.json"), claudeSettings(f.bin))

	f.storeQuota(t, 20*time.Minute)
	rep := buildDoctorReport(f.env)
	c := f.check(t, rep, "claude-code", "quota snapshot")
	wantStatus(t, c, doctorWarn)
	if !strings.Contains(c.Detail, "20m ago") || !strings.Contains(c.Detail, "resume on their own in 40m") {
		t.Errorf("detail should carry the age and when it clears: %q", c.Detail)
	}

	// Past one hour the gate ignores the snapshot, so doctor must not warn.
	f.storeQuota(t, 61*time.Minute)
	if hasCheck(buildDoctorReport(f.env), "claude-code", "quota snapshot") {
		t.Error("an abandoned snapshot no longer holds; doctor must stay quiet")
	}

	// Required mode keeps holding at any age.
	f.storeQuota(t, 5*time.Hour)
	t.Setenv("DOWNSHIFT_QUOTA_MODE", "required")
	f.env.Getenv = func(k string) string {
		if k == "DOWNSHIFT_QUOTA_MODE" {
			return "required"
		}
		return ""
	}
	c = f.check(t, buildDoctorReport(f.env), "claude-code", "quota snapshot")
	wantStatus(t, c, doctorWarn)
	if !strings.Contains(c.Detail, "held until you refresh it") {
		t.Errorf("required mode detail: %q", c.Detail)
	}
}
