// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/quota"
)

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
