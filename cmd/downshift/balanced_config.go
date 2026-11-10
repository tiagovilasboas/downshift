// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
)

// maxBalancedThresholdKB caps balanced_threshold_kb: past the Compactor's hold
// limit nothing is held to cut.
const maxBalancedThresholdKB = compressor.MaxProcessableBytes >> 10

// balancedSettings is the balanced cut in effect: whether it is on, the size
// above which it cuts, and its line caps (zero fields keep their defaults).
type balancedSettings struct {
	on        bool
	threshold int // bytes
	limits    compressor.BalanceLimits
}

// balancedFrom reads the balanced keys of the config file once. env is
// DOWNSHIFT_BALANCED. A config that cannot be read keeps balanced on (as
// contextopt.BalancedEnabled) with the default limits.
func balancedFrom(cfg contextopt.Config, cfgErr error, env string) balancedSettings {
	s := balancedSettings{on: contextopt.BalancedEnabled(cfg, cfgErr, env), threshold: execwrap.BalanceThreshold}
	if cfgErr != nil {
		s.limits = compressor.DefaultBalanceLimits()
		return s
	}
	if kb := cfg.BalancedThresholdKB; kb > 0 {
		s.threshold = min(kb, maxBalancedThresholdKB) << 10
	}
	s.limits = compressor.BalanceLimits{
		HunkLines: cfg.BalancedHunkLines, DiffLines: cfg.BalancedDiffLines,
		MatchesPerFile: cfg.BalancedMatchesPerFile, MatchLines: cfg.BalancedMatchLines,
		LogHead: cfg.BalancedLogHead, LogTail: cfg.BalancedLogTail,
	}.WithDefaults()
	return s
}

// loadBalanced is balancedFrom for the config file and environment.
func loadBalanced() balancedSettings {
	cfg, err := contextopt.LoadConfig()
	return balancedFrom(cfg, err, os.Getenv(execwrap.BalancedEnv))
}

// apply sets the balanced options of one run.
func (s balancedSettings) apply(o *execwrap.Options) {
	o.Balanced, o.BalanceThreshold, o.BalanceLimits = s.on, s.threshold, s.limits
}

// describe words the limits in effect for `downshift doctor`.
func (s balancedSettings) describe() string {
	l := s.limits
	return fmt.Sprintf("outputs over %d KB are cut (log: first %d and last %d lines; diff: %d lines per hunk, %d in all; search: %d matches per file, %d in all), full output via downshift retrieve",
		s.threshold>>10, l.LogHead, l.LogTail, l.HunkLines, l.DiffLines, l.MatchesPerFile, l.MatchLines)
}
