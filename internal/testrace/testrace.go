// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package testrace lets tests with a wall-clock budget stay strict in a normal
// build and tolerate the race detector's slowdown (roughly 5-10x). Import it
// from _test.go files only.
package testrace

import "time"

// Budget returns d, or d times the race slowdown when the detector is on.
// A regression of interest (backtracking, a full scan) is orders of magnitude
// over any budget, so the scaled bound still catches it.
func Budget(d time.Duration) time.Duration { return d * factor }
