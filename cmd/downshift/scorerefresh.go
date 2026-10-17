// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// Public score index refresh trigger (shared layer, no harness names).
//
// The shared hook runner calls maybeRefreshScores after the response is
// written. It costs one stat when the cache is fresh. When scores.json is
// missing or older than scoreMaxAge it starts one detached
// "downshift models scores --refresh --quiet" process, guarded by a lock file
// so parallel hooks start at most one. The hook never waits, never prints and
// never fails because of it. The command line and the file name scores.json
// are the only contract with the scoreindex package.
const (
	scoreCacheFile  = "scores.json"
	scoreLockFile   = "scores-refresh.lock"
	scoreFailedFile = "scores-refresh.failed"
	scoreMaxAge     = 24 * time.Hour
	scoreLockStale  = 10 * time.Minute
	// scoreFailBackoff is how long hooks stop retrying after a refresh that
	// reached no source (offline, firewalled). Without it every hook after
	// the lock went stale spawned another network process.
	scoreFailBackoff = 6 * time.Hour
	scoreEnvSwitch   = "DOWNSHIFT_SCORES"
	scoreStatusOff   = "off"
	scoreStatusNone  = "missing"
)

// scoreRefreshSpawn starts the detached refresh. A variable so tests inject a
// fake and never launch a real process.
var scoreRefreshSpawn = spawnScoreRefresh

// scoreRefreshNow is the clock, replaceable in tests.
var scoreRefreshNow = time.Now

func scoresDisabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(scoreEnvSwitch)), scoreStatusOff)
}

// scoreCacheAge returns the age of scores.json, or ok=false when missing.
func scoreCacheAge() (age time.Duration, ok bool) {
	p, err := paths.Join(scoreCacheFile)
	if err != nil {
		return 0, false
	}
	fi, err := os.Stat(p)
	if err != nil {
		return 0, false
	}
	return scoreRefreshNow().Sub(fi.ModTime()), true
}

// maybeRefreshScores starts a detached refresh when the cache is stale. It
// reports whether it started one. It never panics and never blocks.
func maybeRefreshScores() (started bool) {
	defer func() {
		if recover() != nil {
			started = false
		}
	}()
	if scoresDisabled() {
		return false
	}
	if age, ok := scoreCacheAge(); ok && age < scoreMaxAge {
		return false
	}
	if age, ok := scoreFailedAge(); ok && age < scoreFailBackoff {
		return false
	}
	if !acquireScoreLock() {
		return false
	}
	// A spawn error keeps the lock: the next attempt waits for the stale
	// timeout instead of retrying on every hook.
	return scoreRefreshSpawn() == nil
}

// scoreFailedAge is the age of the last failed-refresh stamp, if any.
func scoreFailedAge() (time.Duration, bool) {
	p, err := paths.Join(scoreFailedFile)
	if err != nil {
		return 0, false
	}
	fi, err := os.Stat(p)
	if err != nil {
		return 0, false
	}
	return scoreRefreshNow().Sub(fi.ModTime()), true
}

// markScoreRefreshFailed writes the stamp that holds hooks back; clear
// removes it after a refresh that succeeded.
func markScoreRefreshFailed(failed bool) {
	p, err := paths.Join(scoreFailedFile)
	if err != nil {
		return
	}
	if !failed {
		_ = os.Remove(p)
		return
	}
	_ = os.WriteFile(p, []byte(scoreRefreshNow().UTC().Format(time.RFC3339)+"\n"), 0o600)
}

// acquireScoreLock creates the lock file exclusively. A lock older than
// scoreLockStale is treated as abandoned and replaced once.
func acquireScoreLock() bool {
	p, err := paths.Join(scoreLockFile)
	if err != nil {
		return false
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return true
		}
		if !os.IsExist(err) {
			return false
		}
		fi, serr := os.Stat(p)
		if serr != nil || scoreRefreshNow().Sub(fi.ModTime()) < scoreLockStale {
			return false
		}
		if os.Remove(p) != nil {
			return false
		}
	}
	return false
}

// spawnScoreRefresh runs this binary detached: own session, stdio to the null
// device, not waited on.
func spawnScoreRefresh() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "models", "scores", "--refresh", "--quiet")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	detachProcess(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// writeScoreStatus is the doctor line. When the cache is stale it starts a
// refresh, same path as the hook.
func writeScoreStatus(w io.Writer) {
	if scoresDisabled() {
		fmt.Fprintf(w, "scores: off (%s=off, no network)\n", scoreEnvSwitch)
		return
	}
	p, _ := paths.Join(scoreCacheFile)
	age, ok := scoreCacheAge()
	switch {
	case ok && age < scoreMaxAge:
		fmt.Fprintf(w, "scores: fresh at %s (updated %s ago)\n", p, age.Round(time.Minute))
	case ok:
		started := maybeRefreshScores()
		fmt.Fprintf(w, "scores: stale at %s (updated %s ago; refresh %s)\n", p, age.Round(time.Minute), refreshWord(started))
	default:
		started := maybeRefreshScores()
		fmt.Fprintf(w, "scores: %s at %s (refresh %s; or run `downshift models scores --refresh`)\n", scoreStatusNone, p, refreshWord(started))
	}
}

func refreshWord(started bool) string {
	if started {
		return "started in background"
	}
	return "not started"
}
