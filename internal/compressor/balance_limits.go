// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

// BalanceLimits are the line caps of the balanced cut. The caller reads them
// from its own configuration and passes them in; the compressor reads no
// config. A zero or negative field keeps its default (DefaultBalanceLimits).
type BalanceLimits struct {
	HunkLines      int // body lines shown per diff hunk
	DiffLines      int // body lines shown across a diff
	MatchesPerFile int // matches shown per file
	MatchLines     int // matches shown in total
	LogHead        int // first lines of a log
	LogTail        int // last lines of a log
}

// DefaultBalanceLimits returns the built-in caps.
func DefaultBalanceLimits() BalanceLimits {
	return BalanceLimits{
		HunkLines:      balanceHunkLines,
		DiffLines:      balanceDiffLines,
		MatchesPerFile: balanceMatchPerFile,
		MatchLines:     balanceMatchLines,
		LogHead:        balanceLogHead,
		LogTail:        balanceLogTail,
	}
}

// WithDefaults returns l with every unset field set to its default.
func (l BalanceLimits) WithDefaults() BalanceLimits {
	d := DefaultBalanceLimits()
	for _, f := range []struct {
		v   *int
		def int
	}{
		{&l.HunkLines, d.HunkLines}, {&l.DiffLines, d.DiffLines},
		{&l.MatchesPerFile, d.MatchesPerFile}, {&l.MatchLines, d.MatchLines},
		{&l.LogHead, d.LogHead}, {&l.LogTail, d.LogTail},
	} {
		if *f.v <= 0 {
			*f.v = f.def
		}
	}
	return l
}
