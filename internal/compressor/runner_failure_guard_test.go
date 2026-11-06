// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// Runner failure-summary guard (BN-5). Every runner format that compacts a
// clean run needs at least one failing fixture (a preserve entry in the
// MANIFEST whose body carries a failure marker). That fixture must stay
// byte-identical at exit 0, at unknown (-1) and at every non-zero exit, so a
// failing body is never summarised away. Removing a runner's failing fixture
// fails TestRunnerHasFailingFixture.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// runnerFormats are the formats of test runners and build tools whose failures
// the agent must read in full.
var runnerFormats = []compressor.Format{
	compressor.FormatGoTest, compressor.FormatGoTestRuns, compressor.FormatPytest,
	compressor.FormatVitest, compressor.FormatJest, compressor.FormatPlaywright,
	compressor.FormatCargoTest, compressor.FormatCypress, compressor.FormatPHPUnit,
	compressor.FormatFlutterTest, compressor.FormatGradle, compressor.FormatMaven,
	compressor.FormatTSC, compressor.FormatNextBuild, compressor.FormatBazel,
	compressor.FormatNx, compressor.FormatTurbo,
}

// failingFixtures returns, per runner format, the fixtures that must stay raw:
// preserve entries with a failure marker, plus the rawFixtures the manifest
// assigns to the format.
func failingFixtures(t *testing.T) (failing map[compressor.Format][]string, withClean map[compressor.Format]bool) {
	t.Helper()
	isRunner := map[compressor.Format]bool{}
	for _, f := range runnerFormats {
		isRunner[f] = true
	}
	raw := map[string]bool{}
	for _, name := range rawFixtures {
		raw[name] = true
	}
	failing, withClean = map[compressor.Format][]string{}, map[compressor.Format]bool{}
	for _, e := range loadManifest(t) {
		if !isRunner[e.format] {
			continue
		}
		switch {
		case e.kind == kindClean:
			withClean[e.format] = true
		case e.kind == kindPreserve && (raw[e.file] || len(dangerSignals(readFixture(t, e.file))) > 0):
			failing[e.format] = append(failing[e.format], e.file)
		}
	}
	return failing, withClean
}

// missingFailingFixture is the explicit list of runner formats that still lack
// a failing fixture. Each entry names the gap; the test fails when an entry is
// stale (the fixture now exists), so the list only shrinks.
var missingFailingFixture = map[compressor.Format]string{}

func TestRunnerHasFailingFixture(t *testing.T) {
	failing, withClean := failingFixtures(t)
	for _, f := range runnerFormats {
		reason, known := missingFailingFixture[f]
		switch {
		case withClean[f] && len(failing[f]) == 0 && !known:
			t.Errorf("runner format %q has a clean fixture but no failing (preserve) fixture with a failure marker in MANIFEST", f)
		case known && len(failing[f]) > 0:
			t.Errorf("runner format %q now has a failing fixture; remove it from missingFailingFixture (%s)", f, reason)
		}
	}
	for f := range missingFailingFixture {
		if !withClean[f] {
			t.Errorf("missingFailingFixture names %q, which has no clean fixture; remove the entry", f)
		}
	}
}

func TestRunnerFailingFixtureStaysRawAtEveryExit(t *testing.T) {
	failing, _ := failingFixtures(t)
	exits := append([]int{-1, 0}, nonZeroExits...)
	for _, f := range runnerFormats {
		for _, file := range failing[f] {
			in := readFixture(t, file)
			for _, code := range exits {
				t.Run(fmt.Sprintf("%s/%s/exit=%d", f, file, code), func(t *testing.T) {
					res := compressFor(in, f, code)
					if res.Applied || !bytes.Equal(res.Output, in) {
						t.Fatalf("failing fixture changed (applied=%v reason=%q)\n got: %q", res.Applied, res.Reason, res.Output)
					}
				})
			}
		}
	}
}
