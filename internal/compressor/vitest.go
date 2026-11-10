// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// Vitest clean runs. A transcript is read line by line against a closed
// grammar: the RUN header, one line per test file, the Test Files / Tests /
// Start at / Duration summary, blank lines, and the notices the Node runtime
// and jsdom print while the run goes on (the wrapper merges vitest's stderr
// in). A clean run folds into one PASS line, followed by every notice it
// printed, each distinct notice once with its count. Any other line is console
// output a test printed (a "stdout | file > test" block or a stray line): up to
// 20 such lines stay verbatim; above that they are dropped and counted, unless
// one carries a danger word or "warn", which stays.
// A run with no Tests summary stays raw. The shapes come from real vitest 3, 4 and 5 runs.

const (
	vitestExtraCount = `\d+ (?:skipped|todo)`
	vitestSpan       = `\d+(?:\.\d+)?(?:ms|s)`
)

var (
	reVitestTests  = lazyre.New(`(?m)^\s*Tests\s+(\d+)\s+passed`)
	reVitestFiles  = lazyre.New(`(?m)^\s*Test Files\s+(\d+)\s+passed`)
	reVitestHeader = lazyre.New(`(?m)^\s*RUN\s+v\d+\.\d+`)
	reVitestFail   = lazyre.New(`(?m)^\s*(?:Test Files|Tests)\s+.*failed`)
	reVitestError  = lazyre.New(`(?:FAIL|Error:|AssertionError)`)

	reVitestHeaderLine = lazyre.New(`^\s*RUN\s+v\d+\.\d+\.\d+(?:\s+\S+)?$`)
	// One test file: "✓ path (5)" (vitest 1), "✓ path (6 tests) 2ms" (vitest 2
	// to 5), "|project|" before the path in a workspace, and "↓ path (3 tests |
	// 3 skipped)" for a file whose tests were all skipped.
	reVitestFileLine = lazyre.New(`^\s*[✓↓]\s+(?:\|[\w@./-]+\|\s+)?\S+\s+\((?:\d+|\d+ tests?(?: \| ` +
		vitestExtraCount + `)*)\)(?:\s+` + vitestSpan + `)?$`)
	reVitestSummaryLine = lazyre.New(`^\s*(Test Files|Tests)\s+(\d+ passed(?: \| ` + vitestExtraCount + `)*)\s+\((\d+)\)$`)
	reVitestStartLine   = lazyre.New(`^\s*Start at\s+\d{2}:\d{2}:\d{2}$`)
	// "Duration 9.14s (transform 3.10s, setup 9.12s, ...)"; vitest 5 gives the
	// breakdown in percent.
	reVitestDurationLine = lazyre.New(`^\s*Duration\s+(` + vitestSpan + `)(?:\s+\(\w+ (?:` + vitestSpan +
		`|\d+%)(?:, \w+ (?:` + vitestSpan + `|\d+%))*\))?$`)
	// Runtime notices a clean run can print: a Node warning, its "(Use `node
	// --trace-... ...`)" hint, and a jsdom "Not implemented" message.
	reVitestNotice = lazyre.New("^(?:\\(node:\\d+\\) (?:\\[[A-Z]+\\d+\\] )?[A-Za-z]*Warning: .+" +
		"|\\(Use `node --trace-(?:warnings|deprecation) \\.\\.\\.` to show where the warning was created\\)" +
		"|Not implemented: .+)$")
	reNodePID = lazyre.New(`^\(node:\d+\)`)
)

func isVitest(input []byte) bool {
	return reVitestTests.Match(input) ||
		reVitestFiles.Match(input) ||
		reVitestHeader.Match(input) ||
		reVitestFail.Match(input)
}

// vitestRun is what a clean transcript states: the Tests counts ("6 passed |
// 1 skipped"), the number of test files, the total duration, and its notices
// in the order first seen.
type vitestRun struct {
	tests, files, span string
	notices            []string       // first occurrence of each distinct notice
	seen               map[string]int // notice without its process id -> count
	console            []consoleLine  // console lines, in order
	inStream           bool           // inside a "stdout | ..." block
}

// add reads one line. It returns a reason when the line keeps the transcript raw.
func (r *vitestRun) add(line string) string {
	if r.inStream && strings.TrimSpace(line) != "" {
		r.tolerate(line)
		return ""
	}
	r.inStream = false
	switch {
	case strings.TrimSpace(line) == "", reVitestHeaderLine.MatchString(line),
		reVitestFileLine.MatchString(line), reVitestStartLine.MatchString(line):
	case reVitestSummaryLine.MatchString(line):
		m := reVitestSummaryLine.FindStringSubmatch(line)
		count, value := &r.files, m[3] // every file counts, skipped ones too
		if m[1] == "Tests" {
			count, value = &r.tests, m[2]
		}
		if *count != "" {
			return "vitest_multiple_runs_preserved"
		}
		*count = value
	case reVitestDurationLine.MatchString(line):
		if r.span != "" {
			return "vitest_multiple_runs_preserved"
		}
		r.span = reVitestDurationLine.FindStringSubmatch(line)[1]
	case reVitestNotice.MatchString(line), reNpmWarnLine.MatchString(line):
		key := reNodePID.ReplaceAllString(line, "(node)")
		if r.seen[key] == 0 {
			r.notices = append(r.notices, line)
		}
		r.seen[key]++
	case reVitestStreamHeader.MatchString(line):
		r.inStream = true
		r.tolerate(line)
	default:
		r.tolerate(line)
	}
	return ""
}

// tolerate records a console line; a line with a danger word or "warn" always
// stays, the others stay verbatim unless there are too many (foldConsole).
func (r *vitestRun) tolerate(line string) {
	r.console = append(r.console, consoleLine{line, reKeepLine.MatchString(line)})
}

// compressVitest folds a clean vitest run into one PASS line and keeps every
// notice: "Vitest: PASS (N passed[, M skipped] across F files[ in T])". If the
// summary is not strictly shorter than the input, the input is returned.
func compressVitest(input []byte) ([]byte, bool, string) {
	run := vitestRun{seen: map[string]int{}}
	body := dropScriptBanner(input) // npm and pnpm wrap the run in a script banner
	for _, line := range strings.Split(strings.TrimSuffix(string(body), "\n"), "\n") {
		if reason := run.add(line); reason != "" {
			return input, false, reason
		}
	}
	if run.tests == "" {
		return input, false, "vitest_missing_tests_summary"
	}
	passed := strings.ReplaceAll(run.tests, " | ", ", ")
	files := "1"
	if run.files != "" {
		files = run.files
	}
	var b strings.Builder
	if run.span != "" {
		fmt.Fprintf(&b, "Vitest: PASS (%s across %s files in %s)\n", passed, files, run.span)
	} else {
		fmt.Fprintf(&b, "Vitest: PASS (%s across %s files)\n", passed, files)
	}
	for _, n := range run.notices {
		b.WriteString(n)
		if c := run.seen[reNodePID.ReplaceAllString(n, "(node)")]; c > 1 {
			fmt.Fprintf(&b, " [repeated %d times]", c)
		}
		b.WriteByte('\n')
	}
	kept, dropped := foldConsole(run.console)
	for _, l := range kept {
		b.WriteString(l + "\n")
	}
	if dropped > 0 {
		fmt.Fprintf(&b, "[%d console output lines dropped]\n", dropped)
	}
	if b.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(b.String()), true, fmt.Sprintf("compressed vitest summary (%s across %s files)", passed, files)
}
