// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/ccr"
	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/testenv"
)

// G4: each balanced key of the config file reaches the Compactor's options;
// an unset, zero or negative key keeps its default, and a config that cannot
// be read keeps every default.
func TestBalancedFromConfig(t *testing.T) {
	def := compressor.DefaultBalanceLimits()
	with := func(f func(*compressor.BalanceLimits)) compressor.BalanceLimits {
		l := def
		f(&l)
		return l
	}
	cases := []struct {
		name          string
		cfg           contextopt.Config
		err           error
		wantThreshold int
		wantLimits    compressor.BalanceLimits
	}{
		{"defaults", contextopt.Config{}, nil, execwrap.BalanceThreshold, def},
		{"threshold", contextopt.Config{BalancedThresholdKB: 64}, nil, 64 << 10, def},
		{"threshold capped at the hold limit", contextopt.Config{BalancedThresholdKB: 1 << 20}, nil, compressor.MaxProcessableBytes, def},
		{"negative threshold keeps default", contextopt.Config{BalancedThresholdKB: -3}, nil, execwrap.BalanceThreshold, def},
		{"hunk lines", contextopt.Config{BalancedHunkLines: 5}, nil, execwrap.BalanceThreshold, with(func(l *compressor.BalanceLimits) { l.HunkLines = 5 })},
		{"diff lines", contextopt.Config{BalancedDiffLines: 90}, nil, execwrap.BalanceThreshold, with(func(l *compressor.BalanceLimits) { l.DiffLines = 90 })},
		{"matches per file", contextopt.Config{BalancedMatchesPerFile: 4}, nil, execwrap.BalanceThreshold, with(func(l *compressor.BalanceLimits) { l.MatchesPerFile = 4 })},
		{"match lines", contextopt.Config{BalancedMatchLines: 40}, nil, execwrap.BalanceThreshold, with(func(l *compressor.BalanceLimits) { l.MatchLines = 40 })},
		{"log head", contextopt.Config{BalancedLogHead: 10}, nil, execwrap.BalanceThreshold, with(func(l *compressor.BalanceLimits) { l.LogHead = 10 })},
		{"log tail", contextopt.Config{BalancedLogTail: 20}, nil, execwrap.BalanceThreshold, with(func(l *compressor.BalanceLimits) { l.LogTail = 20 })},
		{"negative cap keeps default", contextopt.Config{BalancedLogTail: -1}, nil, execwrap.BalanceThreshold, def},
		{"unreadable config", contextopt.Config{BalancedLogTail: 20}, errors.New("bad json"), execwrap.BalanceThreshold, def},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := balancedFrom(tc.cfg, tc.err, "")
			if !s.on || s.threshold != tc.wantThreshold || s.limits != tc.wantLimits {
				t.Fatalf("got %+v, want threshold %d limits %+v", s, tc.wantThreshold, tc.wantLimits)
			}
			var o execwrap.Options
			s.apply(&o)
			if !o.Balanced || o.BalanceThreshold != tc.wantThreshold || o.BalanceLimits != tc.wantLimits {
				t.Fatalf("options %+v", o)
			}
		})
	}
	if balancedFrom(contextopt.Config{}, nil, "0").on {
		t.Error("DOWNSHIFT_BALANCED=0 must turn balanced off")
	}
}

// The config keys are read from the file, and `downshift doctor` prints the
// values in effect on its compactor mode line.
func TestDoctorPrintsBalancedLimits(t *testing.T) {
	home := t.TempDir()
	testenv.SetHome(t, home)
	state := filepath.Join(home, ".downshift")
	t.Setenv("DOWNSHIFT_STATE_DIR", state)
	t.Setenv(execwrap.BalancedEnv, "")
	t.Setenv("DOWNSHIFT_CCR", "")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := `{"balanced_threshold_kb": 32, "balanced_hunk_lines": 7, "balanced_diff_lines": 70, "balanced_matches_per_file": 3, "balanced_match_lines": 33, "balanced_log_head": 11, "balanced_log_tail": 22}`
	if err := os.WriteFile(filepath.Join(state, contextopt.DefaultConfigFileName), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	rep := &doctorReport{}
	checkCompactorMode(rep)
	if len(rep.Checks) != 1 {
		t.Fatalf("checks = %+v", rep.Checks)
	}
	got := rep.Checks[0].Detail
	for _, want := range []string{"over 32 KB", "first 11 and last 22 lines", "7 lines per hunk, 70 in all", "3 matches per file, 33 in all"} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor detail lacks %q: %s", want, got)
		}
	}
}

// Every passthrough reason the exec log can hold has a label in report, and
// the spool limit matches what one CCR entry may hold.
func TestPassthroughReasonsHaveLabels(t *testing.T) {
	for _, r := range []string{
		execwrap.PassthroughSmallOutput, execwrap.PassthroughNonZeroExit, execwrap.PassthroughUnrecognized,
		execwrap.PassthroughRefused, execwrap.PassthroughTooLarge, execwrap.PassthroughStderr,
		execwrap.PassthroughHoldTimeout, execwrap.PassthroughNotRecorded,
	} {
		if notCompactedLabel[r] == "" {
			t.Errorf("reason %q has no label", r)
		}
	}
	if execwrap.SpoolLimit != ccr.DefaultMaxEntryBytes {
		t.Errorf("SpoolLimit %d != ccr.DefaultMaxEntryBytes %d", execwrap.SpoolLimit, ccr.DefaultMaxEntryBytes)
	}
}
