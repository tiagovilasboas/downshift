// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// mavenPath is one path token: it must start with "/", "./" or a drive letter,
// and holds no spaces or other characters. Anything else is not a path.
const mavenPath = `(?:/|\./|[A-Za-z]:[\\/])[A-Za-z0-9_.\-/+@:~\\]*`

// mavenJavacOptions is the closed set of bracketed javac options Maven prints,
// for example "[debug release 17]" or "[debug target 17 -parameters]".
const mavenJavacOptions = `(?:debug|release|source|target|parameters|nowarn|deprecation|encoding)(?: \d+)?(?: (?:debug|release|source|target|parameters|nowarn|deprecation|encoding)(?: \d+)?)*`

const mavenScanLine = "[INFO] Scanning for projects..."

var (
	reMavenBlank     = regexp.MustCompile(`^\[INFO\] ?$`)
	reMavenRule      = regexp.MustCompile(`^\[INFO\] -{20,}$`)
	reMavenProject   = regexp.MustCompile(`^\[INFO\] -+< [A-Za-z0-9_.\-]+:[A-Za-z0-9_.\-]+ >-+$`)
	reMavenBuilding  = regexp.MustCompile(`^\[INFO\] Building ([a-z][a-z0-9.\-]*) (\d+(?:\.[0-9A-Za-z]+)+(?:-[0-9A-Za-z.]+)?)$`)
	reMavenFrom      = regexp.MustCompile(`^\[INFO\]   from [A-Za-z0-9_.\-/]+$`)
	reMavenPackaging = regexp.MustCompile(`^\[INFO\] -+\[ (jar|pom|war|ear|maven-plugin|bundle) \]-+$`)
	reMavenGoal      = regexp.MustCompile(`^\[INFO\] --- ([a-z][a-z0-9.\-]*):(\d+(?:\.[0-9A-Za-z]+)*(?:-[0-9A-Za-z.]+)?):([a-z][A-Za-z0-9]*) \([a-z][A-Za-z0-9\-]*\) @ [A-Za-z0-9_.\-]+ ---$`)
	// reMavenChatter lists the goal chatter that may precede the Surefire
	// header. Only the javac option tokens of a real compile line are accepted,
	// and every path or copy target is one path token.
	reMavenChatter = regexp.MustCompile(`^\[INFO\] (?:Nothing to compile - all classes are up to date\.?` +
		`|Changes detected - recompiling the module!(?: :(?:source|resources))?` +
		`|Compiling \d+ source files? with javac \[` + mavenJavacOptions + `\] to ` + mavenPath +
		`|Copying \d+ resources?(?: from ` + mavenPath + ` to ` + mavenPath + `)?` +
		`|Using '[A-Za-z0-9\-]+' encoding to copy filtered resources\.` +
		`|skip non existing resourceDirectory ` + mavenPath +
		`|Using auto detected provider org\.apache\.maven\.surefire\.[A-Za-z0-9.]+)$`)
	// reMavenPostTests lists the only chatter allowed between the Surefire
	// totals and BUILD SUCCESS: a packaged jar or an install step.
	reMavenPostTests = regexp.MustCompile(`^\[INFO\] (?:Building jar: ` + mavenPath +
		`|Installing ` + mavenPath + ` to ` + mavenPath + `)$`)
	reMavenTestsHeader = regexp.MustCompile(`^\[INFO\]  T E S T S$`)
	reMavenRunning     = regexp.MustCompile(`^\[INFO\] Running (\S+)$`)
	reMavenClassTests  = regexp.MustCompile(`^\[INFO\] Tests run: (\d+), Failures: (\d+), Errors: (\d+), Skipped: (\d+), Time elapsed: [0-9.,]+ (?:s|sec) -- in (\S+)$`)
	reMavenResults     = regexp.MustCompile(`^\[INFO\] Results:$`)
	reMavenTotalTests  = regexp.MustCompile(`^\[INFO\] Tests run: (\d+), Failures: (\d+), Errors: (\d+), Skipped: (\d+)$`)
	reMavenSuccess     = regexp.MustCompile(`^\[INFO\] BUILD SUCCESS$`)
	reMavenTotalTime   = regexp.MustCompile(`^\[INFO\] Total time: +(?:\d+(?:\.\d+)? s|\d+:\d\d min)$`)
	reMavenFinished    = regexp.MustCompile(`^\[INFO\] Finished at: \d{4}-\d\d-\d\dT[0-9:.]+(?:Z|[+-]\d\d:\d\d)$`)
	reMavenReactor     = regexp.MustCompile(`Reactor (?:Build Order|Summary)`)

	// reMavenFailure marks output that must never be summarised: any warning
	// or error line, a failed build, or a Surefire line with failures/errors.
	reMavenFailure = regexp.MustCompile(`(?m)^\[(?:WARNING|ERROR)\]|BUILD FAILURE|Failures: [1-9]|Errors: [1-9]|\bFAIL`)
)

// isMaven recognises Maven's own first console line. Nothing else prints it.
func isMaven(input []byte) bool {
	first, _, _ := strings.Cut(string(input), "\n")
	return first == mavenScanLine
}

// compressMaven compacts only a complete, single-module, all-pass Maven 3.9
// transcript made of lines it fully understands: per-goal section chatter is
// reduced to one line per goal and the Surefire total, BUILD SUCCESS and Total
// time lines are kept verbatim. Warnings, errors, downloads, skipped tests,
// test output, reactor builds and any unknown line return the input unchanged.
func compressMaven(input []byte) ([]byte, bool, string) {
	text := string(input)
	if len(input) == 0 || input[len(input)-1] != '\n' || strings.ContainsAny(text, "\r\x1b") {
		return input, false, "maven_incomplete_bytes_preserved"
	}
	if reMavenReactor.MatchString(text) {
		return input, false, "maven_multi_module_preserved"
	}

	var (
		goals                                  []string
		name, version, packaging               string
		totalLine, successLine, timeLine       string
		seenProject, seenBuilding, seenSuccess bool
		seenFinished, seenResults, seenHeader  bool
		sumRun                                 int
		pendingClass                           string
		totalRun                               = -1
	)

	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	for i, line := range lines {
		if i == 0 {
			if line != mavenScanLine {
				return input, false, "maven_unrecognized_output_preserved"
			}
			continue
		}
		if seenSuccess {
			switch {
			case reMavenRule.MatchString(line), reMavenBlank.MatchString(line):
			case reMavenTotalTime.MatchString(line) && timeLine == "":
				timeLine = line
			case reMavenFinished.MatchString(line) && !seenFinished && timeLine != "":
				seenFinished = true
			default:
				return input, false, "maven_unrecognized_output_preserved"
			}
			continue
		}
		switch {
		case reMavenBlank.MatchString(line), reMavenRule.MatchString(line):
		case reMavenProject.MatchString(line):
			if seenProject || len(goals) > 0 {
				return input, false, "maven_multi_module_preserved"
			}
			seenProject = true
		case len(goals) == 0 && reMavenBuilding.MatchString(line):
			if !seenProject || seenBuilding {
				return input, false, "maven_unrecognized_output_preserved"
			}
			m := reMavenBuilding.FindStringSubmatch(line)
			name, version = m[1], m[2]
			seenBuilding = true
		case len(goals) == 0 && reMavenFrom.MatchString(line):
			if !seenBuilding {
				return input, false, "maven_unrecognized_output_preserved"
			}
		case len(goals) == 0 && reMavenPackaging.MatchString(line):
			if !seenBuilding || packaging != "" {
				return input, false, "maven_unrecognized_output_preserved"
			}
			packaging = reMavenPackaging.FindStringSubmatch(line)[1]
		case reMavenGoal.MatchString(line):
			if !seenBuilding || packaging == "" {
				return input, false, "maven_unrecognized_output_preserved"
			}
			m := reMavenGoal.FindStringSubmatch(line)
			goals = append(goals, m[1]+":"+m[3])
		case len(goals) > 0 && !seenHeader && reMavenChatter.MatchString(line):
		case len(goals) > 0 && totalRun >= 0 && reMavenPostTests.MatchString(line):
		case len(goals) > 0 && reMavenTestsHeader.MatchString(line):
			seenHeader = true
		case seenHeader && reMavenRunning.MatchString(line):
			if pendingClass != "" || seenResults {
				return input, false, "maven_unrecognized_output_preserved"
			}
			pendingClass = reMavenRunning.FindStringSubmatch(line)[1]
		case seenHeader && reMavenClassTests.MatchString(line):
			m := reMavenClassTests.FindStringSubmatch(line)
			if pendingClass == "" || m[5] != pendingClass || m[2] != "0" || m[3] != "0" || m[4] != "0" || seenResults {
				return input, false, "maven_inconsistent_tests_preserved"
			}
			n, _ := strconv.Atoi(m[1])
			sumRun += n
			pendingClass = ""
		case seenHeader && reMavenResults.MatchString(line):
			if pendingClass != "" || seenResults {
				return input, false, "maven_unrecognized_output_preserved"
			}
			seenResults = true
		case seenResults && totalRun < 0 && reMavenTotalTests.MatchString(line):
			m := reMavenTotalTests.FindStringSubmatch(line)
			if m[2] != "0" || m[3] != "0" || m[4] != "0" {
				return input, false, "maven_inconsistent_tests_preserved"
			}
			totalRun, _ = strconv.Atoi(m[1])
			totalLine = line
		case reMavenSuccess.MatchString(line):
			if len(goals) == 0 || pendingClass != "" || seenHeader != seenResults {
				return input, false, "maven_incomplete_or_inconsistent_success_preserved"
			}
			successLine = line
			seenSuccess = true
		default:
			return input, false, "maven_unrecognized_output_preserved"
		}
	}

	if !seenSuccess || !seenFinished || timeLine == "" ||
		(seenResults && (totalRun < 0 || totalRun != sumRun)) {
		return input, false, "maven_incomplete_or_inconsistent_success_preserved"
	}

	var out strings.Builder
	fmt.Fprintf(&out, "Maven: %s %s (%s)\n", name, version, packaging)
	for _, g := range goals {
		out.WriteString("  " + g + "\n")
	}
	if totalLine != "" {
		out.WriteString(totalLine + "\n")
	}
	out.WriteString(successLine + "\n" + timeLine + "\n")
	if out.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out.String()), true, fmt.Sprintf("compressed Maven successful build (%d goals)", len(goals))
}
