// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reGradleTask    = regexp.MustCompile(`^> Task (:[A-Za-z0-9_.:\-]*)(?: (UP-TO-DATE|NO-SOURCE|FROM-CACHE|SKIPPED))?$`)
	reGradleBuild   = regexp.MustCompile(`^BUILD SUCCESSFUL in [0-9][0-9a-z ]*$`)
	reGradleAnyEnd  = regexp.MustCompile(`(?m)^BUILD (?:SUCCESSFUL|FAILED) in `)
	reGradleAnyTask = regexp.MustCompile(`(?m)^> Task :`)
	reGradleActions = regexp.MustCompile(`^\d+ actionable tasks?: \d+ (?:executed|up-to-date|from cache)(?:, \d+ (?:executed|up-to-date|from cache))*$`)
	reGradleDaemon  = regexp.MustCompile(`^Starting a Gradle Daemon.*$`)

	// reGradleFailure marks transcripts that are never compacted: failed
	// builds and failed tasks.
	reGradleFailure = regexp.MustCompile(`(?m)^BUILD FAILED|^FAILURE:|\bFAILED\b`)
)

// isGradle needs both a Gradle task line and a Gradle build result line, so
// no other tool's output is claimed.
func isGradle(input []byte) bool {
	return reGradleAnyEnd.Match(input) && reGradleAnyTask.Match(input)
}

// compressGradle compacts only Gradle's plain-console clean success
// transcript: optional daemon start line, task lines, BUILD SUCCESSFUL and the
// actionable-tasks line. Warnings, compiler diagnostics (w:/e:), deprecation
// notices, test output, ANSI colour and any unknown line return the input
// unchanged.
func compressGradle(input []byte) ([]byte, bool, string) {
	s := string(input)
	if len(input) == 0 || input[len(input)-1] != '\n' || strings.ContainsRune(s, '\r') {
		return input, false, "gradle_incomplete_bytes_preserved"
	}
	if strings.ContainsRune(s, '\x1b') {
		return input, false, "gradle_ansi_preserved"
	}
	var (
		startup         []string
		build, actions  string
		tasks, executed int
		upToDate        int
		other           = map[string]int{}
	)
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		switch {
		case reGradleDaemon.MatchString(line):
			if tasks > 0 || build != "" {
				return input, false, "gradle_unexpected_startup_line_preserved"
			}
			startup = append(startup, line)
		case reGradleTask.MatchString(line):
			if build != "" {
				return input, false, "gradle_task_after_result_preserved"
			}
			tasks++
			switch m := reGradleTask.FindStringSubmatch(line)[2]; m {
			case "":
				executed++
			case "UP-TO-DATE":
				upToDate++
			default:
				other[m]++
			}
		case reGradleBuild.MatchString(line):
			if build != "" || tasks == 0 {
				return input, false, "gradle_ambiguous_result_preserved"
			}
			build = line
		case reGradleActions.MatchString(line):
			if build == "" || actions != "" {
				return input, false, "gradle_unexpected_actionable_line_preserved"
			}
			actions = line
		default:
			return input, false, "gradle_unrecognized_output_preserved"
		}
	}
	if build == "" || actions == "" {
		return input, false, "gradle_incomplete_success_preserved"
	}

	var out strings.Builder
	for _, l := range startup {
		out.WriteString(l + "\n")
	}
	fmt.Fprintf(&out, "gradle: %d tasks (%d executed, %d UP-TO-DATE", tasks, executed, upToDate)
	for _, k := range []string{"NO-SOURCE", "FROM-CACHE", "SKIPPED"} {
		if other[k] > 0 {
			fmt.Fprintf(&out, ", %d %s", other[k], k)
		}
	}
	out.WriteString(")\n" + build + "\n" + actions + "\n")
	if out.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out.String()), true, fmt.Sprintf("compressed Gradle success (%d tasks)", tasks)
}
