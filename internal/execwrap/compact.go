// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"fmt"
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// How a finished run's held output is compacted: which compressor reads it,
// and why it stays raw when it does. run.go holds the process and the gate.

// compactOutput returns the bytes to print and, when they are raw, why. A
// failed run goes through the family's failure compressor (CompressFailed):
// every failure line is kept and only passing noise is cut; when that is not
// possible the run passes through whole, reason non-zero exit. A failed run
// that wrote to stderr is raw unless its family prints the failure report
// there (failedStderrIsReport, vitest): the gate then holds the merged
// stream in write order and the failure compressor reads it whole. Any other
// family must not fold a stderr line it does not know.
//
// The third result is the passthrough detail (passthrough_detail.go); it is
// set only when the reason is PassthroughUnrecognized.
func compactOutput(exitCode int, data []byte, family Family, wroteStderr bool) ([]byte, string, string) {
	return compactOutputFor(nil, exitCode, data, family, wroteStderr)
}

// compactOutputFor is compactOutput for a run of argv: the families that read
// the command line (file reads, the rg and grep match forms) need it.
func compactOutputFor(argv []string, exitCode int, data []byte, family Family, wroteStderr bool) ([]byte, string, string) {
	switch {
	case len(data) == 0:
		return data, PassthroughSmallOutput, ""
	case !compactibleFamily(family):
		return data, PassthroughUnrecognized, DetailNoCompressor
	case exitCode != 0 && wroteStderr && !failedStderrIsReport[family]:
		return data, PassthroughNonZeroExit, ""
	}
	out, reason, detail := compactSafely(argv, data, family, exitCode)
	if reason != "" {
		if exitCode != 0 {
			reason, detail = PassthroughNonZeroExit, ""
		}
		return data, reason, detail
	}
	return out, "", ""
}

// noGainReason is the compressor's Result.Reason when it recognised a shape
// but the summary would not be shorter. The compressor exports no constant
// for it; the table test in passthrough_test.go pins this value.
const noGainReason = "no_compression_gain"

// compactSafely runs the compressor under recover: a compressor bug prints
// the raw output instead. It returns the summary and marker, or the reason
// the output stays raw. The compressor detects the format; the output is kept
// only when that format is one the family is allowed to compact as (a gh pr
// command, for instance, accepts gh_pr and gh_checks but nothing else). A
// non-zero exit uses the failure compressor, which summarises a failed go
// test, vitest or playwright run and returns every other family's output
// unchanged. A
// family whose output is another program's transcript with a go test run
// inside (make, git commit, git push) has only that run folded.
func compactSafely(argv []string, data []byte, family Family, exitCode int) (out []byte, reason, detail string) {
	defer func() {
		if recover() != nil {
			out, reason, detail = nil, PassthroughUnrecognized, DetailCompressorPanic
		}
	}()
	var r compressor.Result
	switch {
	case formatAccepted(family, compressor.FormatGitRemote):
		r = compressor.CompressGitRemote(data, exitCode)
	case commandFormats[family] != "":
		format, hint := commandPlan(family, argv)
		r = compressor.CompressCommandOutputHint(data, format, exitCode, hint)
	case exitCode != 0:
		r = compressor.CompressFailed(data, failedFormat(family, data))
	case formatAccepted(family, compressor.FormatGoTestRuns):
		r = compressor.FoldGoTestRuns(data)
		if !r.Applied && genericFamilies[family] {
			// No go test run to fold: the generic chain may still apply.
			r = compressor.CompressExit(data, compressor.ModeSafe, 0)
		}
	default:
		r = compressor.CompressExit(data, compressor.ModeSafe, 0)
	}
	if !formatAccepted(family, r.Format) {
		return nil, PassthroughUnrecognized, detailFromReason(r.Reason)
	}
	if wd, err := os.Getwd(); exitCode == 0 && err == nil {
		r = compressor.StripNoise(r, wd)
	}
	if !r.Applied {
		if r.Reason == noGainReason {
			return nil, PassthroughSmallOutput, ""
		}
		// The shape was recognised but kept raw for safety (a danger word, an
		// unknown line): the family's compressor did not summarise it.
		return nil, PassthroughUnrecognized, detailFromReason(r.Reason)
	}
	out = append(append([]byte{}, r.Output...), Marker(len(data), len(r.Output))...)
	if len(out) >= len(data) {
		return nil, PassthroughSmallOutput, ""
	}
	return out, "", ""
}

// Marker is the last line of compacted output. It tells the reader that the
// text above is a summary, not what the program printed, and that no copy of
// the original is kept (keepRaw swaps it for MarkerWithID when one is).
func Marker(rawBytes, summaryBytes int) string {
	return fmt.Sprintf("[downshift compact: %d to %d bytes; original not kept]\n", rawBytes, summaryBytes)
}
