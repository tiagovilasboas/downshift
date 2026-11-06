// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// FormatNx is a clean `nx` run transcript.
const FormatNx Format = "nx"

// An Nx run echoes the command ("> nx run app:build", "> app@1.0.0 build",
// "> tsc"), then prints the NX banner with the result. The compressor cuts the
// echo, the rule lines and the cache notice, and keeps the "Successfully ran"
// result as one line. Any other line is the task's own output, which has no
// known shape here, so it keeps the run raw, as do a warning, an error and a
// failed task.
var (
	reNxDone = regexp.MustCompile(`(?m)^ NX   Successfully ran target (\S+) for (project \S+|\d+ projects)(?: and \d+ tasks? they depend on)? \((\S+)\)\s*$`)

	reNxNoise = regexp.MustCompile(`^(?:` +
		`> .*|[—-]{10,}|` +
		` NX   (?:Running target \S+ for .*|Successfully ran target .*)|` +
		`Nx read the output from the cache instead of running the command for \d+ out of \d+ tasks\.` +
		`)$`)
)

func isNx(input []byte) bool { return reNxDone.Match(input) }

// compressNx reduces a clean nx run to one summary line.
func compressNx(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, nil) {
		return input, false, "nx_danger_preserved"
	}
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.TrimSpace(line) != "" && !reNxNoise.MatchString(line) {
			return input, false, "nx_unrecognised_line_preserved"
		}
	}
	m := reNxDone.FindSubmatch(input)
	out := fmt.Sprintf("[x] nx: ran target %s for %s in %s\n", m[1], m[2], m[3])
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "nx_compacted"
}
