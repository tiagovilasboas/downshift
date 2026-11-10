// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// FormatBazel is a clean `bazel build` or `bazel test` transcript.
const FormatBazel Format = "bazel"

// Bazel writes its report on stderr. The compressor cuts only the lines it can
// name: analysis and action counters, the server start line and one PASSED row
// per test target (the "Executed N out of M tests" line says the same). The
// build summary line, the up-to-date targets with their output paths and the
// test summary survive. A WARNING, an ERROR, a DEBUG line, a FAILED row, a
// progress line or any other line keeps the run raw.
var (
	reBazelDone = lazyre.New(`(?m)^INFO: Build completed successfully, (\d+) total actions?$`)

	reBazelNoise = lazyre.New(`^(?:` +
		`Starting local Bazel server(?: \(\S+\))? and connecting to it\.\.\.|` +
		`INFO: Analy[sz]ed (?:target \S+|\d+ targets?) \(\d+ packages? loaded(?:, \d+ targets? configured)?\)\.|` +
		`INFO: Found .*targets?\.\.\.|` +
		`INFO: \d+ processes?(?:: .*)?\.?|` +
		`INFO: Build completed successfully, \d+ total actions?|` +
		`(//\S+)\s+(?:\(cached\) )?PASSED in [\d.]+s` +
		`)$`)
	reBazelElapsed = lazyre.New(`^INFO: Elapsed time: ([\d.]+s)(?:, Critical Path: [\d.]+s)?$`)
	// reBazelKept lines survive verbatim: the targets built and where their
	// outputs are, and the test summary.
	reBazelKept = lazyre.New(`^(?:Target \S+ up-to-date(?: \(nothing to build\)|:)|  \S+|Executed \d+ out of \d+ tests?: .*)$`)
)

func isBazel(input []byte) bool { return reBazelDone.Match(input) }

// compressBazel reduces a clean bazel build or test run to its summary.
func compressBazel(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, nil) {
		return input, false, "bazel_danger_preserved"
	}
	elapsed := ""
	var kept []string
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		switch m := reBazelElapsed.FindStringSubmatch(line); {
		case line == "" || reBazelNoise.MatchString(line):
		case m != nil:
			elapsed = m[1]
		case reBazelKept.MatchString(line):
			kept = append(kept, line)
		default:
			return input, false, "bazel_unrecognised_line_preserved"
		}
	}
	done := reBazelDone.FindSubmatch(input)
	summary := fmt.Sprintf("[x] bazel: build completed successfully, %s total actions", done[1])
	if elapsed != "" {
		summary += " in " + elapsed
	}
	out := summary + "\n"
	if len(kept) > 0 {
		out += strings.Join(kept, "\n") + "\n"
	}
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "bazel_compacted"
}
