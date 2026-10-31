// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import "github.com/tiagovilasboas/downshift/internal/compressor"

// expectedFormats is the set of compressor formats each family may be
// compacted as. A command of the family is compacted only when the compressor
// recognises its output as one of these formats; any other detected shape (a
// repetitive-log match, a search match) keeps the raw output.
//
// Most families map to a single format. A family whose one command produces
// more than one distinct shape lists them all: gh-run covers both `gh run
// list` (gh_run_list) and `gh run view --log` (gh_run_log); gh-pr covers both
// `gh pr view` (gh_pr) and `gh pr checks` (gh_checks). The compressor detects
// which shape an invocation produced and compacts that one.
//
// A family with no entry here is a pass-through: it runs and its output is
// never compacted (the git families other than git-status, whose clean
// success has no output worth compacting over the marker line, and every gh
// family that returns small ad-hoc text).
var expectedFormats = map[Family][]compressor.Format{
	FamilyGoTest:       {compressor.FormatGoTest},
	FamilyGitStatus:    {compressor.FormatGitStatus},
	FamilyGitDiff:      {compressor.FormatGitDiff},
	FamilyGitShow:      {compressor.FormatGitShow},
	FamilyPytest:       {compressor.FormatPytest},
	FamilyVitest:       {compressor.FormatVitest},
	FamilyPlaywright:   {compressor.FormatPlaywright},
	FamilyCargoTest:    {compressor.FormatCargoTest},
	FamilyTSC:          {compressor.FormatTSC},
	FamilyJest:         {compressor.FormatJest},
	FamilyCypress:      {compressor.FormatCypress},
	FamilyNextBuild:    {compressor.FormatNextBuild},
	FamilyPHPUnit:      {compressor.FormatPHPUnit},
	FamilyFlutterTest:  {compressor.FormatFlutterTest},
	FamilyDartTest:     {compressor.FormatFlutterTest},
	FamilyGradle:       {compressor.FormatGradle},
	FamilyMaven:        {compressor.FormatMaven},
	FamilyESLint:       {compressor.FormatEslint},
	FamilyRuff:         {compressor.FormatRuff},
	FamilyMypy:         {compressor.FormatMypy},
	FamilyGolangciLint: {compressor.FormatGolangciLint},
	FamilyGoBuild:      {compressor.FormatGoBuild},
	FamilyCargoBuild:   {compressor.FormatCargoBuild},
	FamilyDotnetBuild:  {compressor.FormatDotnetBuild},
	FamilyGhRun:        {compressor.FormatGhRunList, compressor.FormatGhRunLog},
	FamilyGhPr:         {compressor.FormatGhPr, compressor.FormatGhChecks},
}

// compactibleFamily reports whether a family may be compacted at all. A
// pass-through family has no entry in expectedFormats.
func compactibleFamily(f Family) bool {
	return len(expectedFormats[f]) > 0
}

// formatAccepted reports whether the compressor's detected format is one the
// family is allowed to be compacted as. A family with no entry accepts
// nothing, so its output always passes through.
func formatAccepted(f Family, got compressor.Format) bool {
	for _, want := range expectedFormats[f] {
		if want == got {
			return true
		}
	}
	return false
}

// failedFormat is the format passed to compressor.CompressFailed for a
// non-zero exit. CompressFailed only summarises a failed go test run and
// returns every other format unchanged, so the one family that can compact a
// failure (go test) passes its format and every other family passes its first
// listed format, which CompressFailed declines. The output then stays raw and
// red, which is what a failing gh check or build must do.
func failedFormat(f Family) compressor.Format {
	formats := expectedFormats[f]
	for _, want := range formats {
		if want == compressor.FormatGoTest {
			return want
		}
	}
	if len(formats) > 0 {
		return formats[0]
	}
	return compressor.FormatUnknown
}
