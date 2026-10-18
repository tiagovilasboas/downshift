// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// After a refresh that reached no source, hooks stop spawning refreshes for
// scoreFailBackoff; an old stamp no longer holds them back.
func TestScoreRefresh_FailedStampBacksOff(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("DOWNSHIFT_SCORES", "")
	n := 0
	old := scoreRefreshSpawn
	scoreRefreshSpawn = func() error { n++; return nil }
	t.Cleanup(func() { scoreRefreshSpawn = old })

	markScoreRefreshFailed(true)
	if maybeRefreshScores() || n != 0 {
		t.Fatalf("fresh failure stamp must hold the refresh, spawned %d", n)
	}

	stamp := filepath.Join(dir, scoreFailedFile)
	oldTime := time.Now().Add(-scoreFailBackoff - time.Minute)
	if err := os.Chtimes(stamp, oldTime, oldTime); err != nil {
		t.Fatal(err)
	}
	if !maybeRefreshScores() || n != 1 {
		t.Fatalf("expired stamp must allow one refresh, spawned %d", n)
	}

	markScoreRefreshFailed(false)
	if _, err := os.Stat(stamp); !os.IsNotExist(err) {
		t.Fatal("a successful refresh must clear the stamp")
	}
}
