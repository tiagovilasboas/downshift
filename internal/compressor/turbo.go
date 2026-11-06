// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// FormatTurbo is a clean `turbo run` transcript.
const FormatTurbo Format = "turbo"

// Turborepo prefixes every task line with "package:task: ". The compressor
// cuts the header bullets, the cache notice, the empty prefixed lines and the
// script echo ("> pkg@1.0.0 build", "> tsc"), and keeps the Tasks/Cached/Time
// summary as one line. Any other prefixed line is the task's own output, which
// has no known shape here, so it keeps the run raw; so does a warning, an error
// or a failed task.
var (
	reTurboTasks  = regexp.MustCompile(`(?m)^\s*Tasks:\s+(\d+) successful, (\d+) total\s*$`)
	reTurboCached = regexp.MustCompile(`(?m)^\s*Cached:\s+(\d+) cached, (\d+) total\s*$`)
	reTurboTime   = regexp.MustCompile(`(?m)^\s*Time:\s+(\S+)\s*(?:>>> FULL TURBO)?\s*$`)

	reTurboNoise = regexp.MustCompile(`^(?:` +
		`• .*|turbo \d+\.\d+\.\d+|` +
		`\s*(?:Tasks|Cached|Time):\s+.*|\s*Failed:.*|` +
		`\S+:\S+: (?:cache (?:miss, executing|hit, replaying logs|bypass, force executing) \w+|)|` +
		`\S+:\S+: > .*` +
		`)$`)
)

func isTurbo(input []byte) bool {
	return reTurboTasks.Match(input) && reTurboCached.Match(input)
}

// compressTurbo reduces a clean turbo run to one summary line.
func compressTurbo(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, nil) {
		return input, false, "turbo_danger_preserved"
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if line != "" && !reTurboNoise.MatchString(line) {
			return input, false, "turbo_unrecognised_line_preserved"
		}
	}
	t, c := reTurboTasks.FindSubmatch(input), reTurboCached.FindSubmatch(input)
	if t[1] == nil || string(t[1]) != string(t[2]) {
		return input, false, "turbo_not_all_successful"
	}
	out := fmt.Sprintf("[x] turbo: %s successful, %s total (%s cached)", t[1], t[2], c[1])
	if m := reTurboTime.FindSubmatch(input); m != nil {
		out += " in " + string(m[1])
	}
	out += "\n"
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "turbo_compacted"
}
