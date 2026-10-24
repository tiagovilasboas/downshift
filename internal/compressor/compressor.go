// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	// MaxProcessableBytes prevents unbounded memory usage.
	MaxProcessableBytes = 2 * 1024 * 1024 // 2 MB

	// pytestPathChars is the closed character set of a pytest path value.
	pytestPathChars = `[A-Za-z0-9_./@+~-]+`
	// pytestPathValue is a relative or absolute path: the closed set, with at
	// least one separator or dot, so a bare word such as FAILED is refused.
	pytestPathValue = `[A-Za-z0-9_@+~-]*[./][A-Za-z0-9_./@+~-]*`
)

var (
	reGoTestPass   = regexp.MustCompile(`(?m)^ok\s+([^\s]+)\s+([^\s]+)(?:\s+coverage:\s+(.+))?$`)
	reGoTestNoTest = regexp.MustCompile(`(?m)^\?\s+([^\s]+)\s+\[no test files\]\s*$`)
	// reGoTestCoverNoFiles is the line `go test -cover` prints for a package
	// with no test files under the tab-indented form (no "ok" or "?" prefix).
	reGoTestCoverNoFiles = regexp.MustCompile(`^\t(\S+)\t+coverage: [\d.]+% of statements$`)
	reGoTestFail         = regexp.MustCompile(`(?m)^(?:--- FAIL:|FAIL|\s*panic:)`)
	reGitLogCommit       = regexp.MustCompile(`(?m)^commit\s+[0-9a-f]{40}`)
	reGitStatusHead      = regexp.MustCompile(`(?m)^(?:On branch |HEAD detached at )`)
	reRipgrepMatch       = regexp.MustCompile(`(?m)^(?:\d+:)?[^:\n]+:\d+:`)
	rePytestSummary      = regexp.MustCompile(`(?m)^=+\s*(?:(\d+)\s+skipped,\s*)?(\d+)\s+passed(?:,\s*(\d+)\s+skipped)?\s+in\s+(\d+(?:\.\d+)?s) =+$`)
	rePytestHeader       = regexp.MustCompile(`(?m)^=+\s*test session starts\s*=+`)
	rePytestFailSummary  = regexp.MustCompile(`(?m)^=+\s*.*?(?:failed|error).*?\s+in\s+`)
	// rePytestPlainLine lists every line shape a clean pytest run emits. A
	// transcript is compacted only when each line matches one of these shapes
	// (or is the single passing summary); any other line keeps the output raw.
	// Path values use one closed character set and no spaces; a value outside
	// it keeps the whole transcript raw.
	rePytestPlainLine = regexp.MustCompile(`^(?:` +
		`|=+ test session starts =+` +
		`|platform [A-Za-z0-9_.-]+ -- Python [0-9][0-9A-Za-z.+-]*(?:, (?:pytest|pluggy)-[0-9][0-9A-Za-z.+-]*)*(?: -- /` + pytestPathChars + `)?` +
		`|rootdir: /` + pytestPathChars +
		`|configfile: ` + pytestPathValue +
		`|cachedir: ` + pytestPathValue +
		`|plugins: [A-Za-z0-9_.-]+-[0-9][A-Za-z0-9_.+-]*(?:, [A-Za-z0-9_.-]+-[0-9][A-Za-z0-9_.+-]*)*` +
		`|collected \d+ items?(?: / \d+ deselected)?` +
		`|` + pytestPathChars + `\.py [.s]+(?: +\[ *\d+%\])?` +
		`)$`)
	reVitestTests    = regexp.MustCompile(`(?m)^\s*Tests\s+(\d+)\s+passed`)
	reVitestFiles    = regexp.MustCompile(`(?m)^\s*Test Files\s+(\d+)\s+passed`)
	reVitestHeader   = regexp.MustCompile(`(?m)^\s*RUN\s+v\d+\.\d+`)
	reVitestFail     = regexp.MustCompile(`(?m)^\s*(?:Test Files|Tests)\s+.*failed`)
	reVitestDuration = regexp.MustCompile(`(?m)^\s*Duration\s+([^\s\n()]+)`)
	reVitestError    = regexp.MustCompile(`(?:FAIL|Error:|AssertionError)`)
	// reVitestPlainLine matches the only lines a clean vitest run prints.
	// Any other line, including an unexpected error, warning or FAIL, makes
	// the whole transcript raw. Recognition is the rule, not a danger list.
	reVitestPlainLine   = regexp.MustCompile(`^(?:\s*|\s*RUN\s+v\d+\.\d+\.\d+(?:\s+\S+)?|\s*✓\s+\S+\s+\(\d+\)|\s*Test Files\s+\d+\s+passed\s+\(\d+\)|\s*Tests\s+\d+\s+passed\s+\(\d+\)|\s*Start at\s+\d{2}:\d{2}:\d{2}|\s*Duration\s+\d+(?:\.\d+)?(?:ms|s)(?:\s+\((?:\w+ \d+ms)(?:, \w+ \d+ms)*\))?)$`)
	rePlaywrightHeader  = regexp.MustCompile(`(?m)^\s*Running\s+(\d+)\s+tests\s+using\s+(\d+)\s+workers?`)
	rePlaywrightSummary = regexp.MustCompile(`(?m)^\s*(\d+)\s+passed\s+\(([^)]+)\)`)
	rePlaywrightError   = regexp.MustCompile(`(?i)(?:failed|flaky|timed out|Error:|AssertionError|trace(?: file| attached)?|retry(?:ing| #))`)
	reCargoTestSummary  = regexp.MustCompile(`(?m)^\s*test result: ok\.\s+(\d+)\s+passed;\s+0\s+failed;.*?finished in\s+([^\n\r]+?)\s*$`)
	reCargoTestHeader   = regexp.MustCompile(`(?m)^running\s+\d+\s+tests`)
	reCargoTestError    = regexp.MustCompile(`(?m)(?:FAILED|error\[E[^\]]*\]:|warning:|panicked at)`)
	reTSCPass           = regexp.MustCompile(`(?m)Found 0 errors`)
	reTSCError          = regexp.MustCompile(`(?m)(?:error\s+TS\d+:|Found\s+[1-9]\d*\s+errors?)`)
	reJestSuites        = regexp.MustCompile(`(?m)^Test Suites:\s+(?:(\d+) skipped,\s*)?(\d+) passed,\s+(\d+) total\s*$`)
	reJestTests         = regexp.MustCompile(`(?m)^Tests:\s+(?:(\d+) skipped,\s*)?(?:(\d+) todo,\s*)?(\d+) passed,\s+(\d+) total\s*$`)
	reJestHeader        = regexp.MustCompile(`(?m)^\s*(?:PASS|Test Suites:)\s+`)
	reJestFail          = regexp.MustCompile(`(?m)^\s*(?:FAIL|Test Suites:.*\bfailed\b|Tests:.*\bfailed\b|Snapshots:.*\bfailed\b)`)
	reJestError         = regexp.MustCompile(`(?m)(?:^\s*Error:|^\s*●|\bSnapshot\s+Summary\b|\bTest suite failed to run\b)`)
	reJestPass          = regexp.MustCompile(`^PASS\s+\S.*$`)
	reJestSnapshots     = regexp.MustCompile(`^Snapshots:\s+(?:(?:\d+ (?:failed|passed|obsolete),\s*)*\d+ total|\d+ total)$`)
	reJestTime          = regexp.MustCompile(`^Time:\s+\d+(?:\.\d+)? s(?:, estimated \d+(?:\.\d+)? s)?$`)
)

var (
	// reTSCPlainLine matches the only lines a clean tsc watch-mode run prints.
	// A project path must end in tsconfig*.json, so no free-text word (error,
	// panic, FAIL) can ride along in a path. Any other line, including an
	// injected danger word or an ANSI-wrapped line, makes the transcript raw.
	reTSCPlainLine = regexp.MustCompile(`^(?:|\[\d{2}:\d{2}:\d{2}\] (?:Projects in this build: ?|Building project(?: '/?(?:[\w@.-]+/)*tsconfig(?:\.(?:app|node|build|base|spec))?\.json')?\.\.\.|Found 0 errors\. Watching for file changes\.)|    \* (?:[\w@.-]+/)*tsconfig(?:\.(?:app|node|build|base|spec))?\.json)$`)
	// reTSCSuccessLine is the 0-errors summary a clean run must end with.
	reTSCSuccessLine = regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\] Found 0 errors\. Watching for file changes\.$`)
)

// Compress evaluates the input according to the policy and selected mode.
// If mode is ModeOff or ModeObserve, the returned Result.Output is byte-identical
// to input. In ModeSafe, recognized safe formats are compacted.
// Compress is CompressExit with an unknown exit code (-1). Text heuristics
// still fail open on error-shaped output, but a caller that knows the
// process exit code must use CompressExit.
func Compress(input []byte, mode Mode) Result {
	return CompressExit(input, mode, -1)
}

// CompressExit preserves the raw output when exitCode is non-zero.
// exitCode < 0 means the caller does not know the status.
func CompressExit(input []byte, mode Mode, exitCode int) Result {
	origLen := len(input)
	res := Result{
		Output:        input,
		Applied:       false,
		Format:        FormatUnknown,
		OriginalBytes: origLen,
		ReducedBytes:  origLen,
	}

	if exitCode > 0 {
		res.Reason = "nonzero_exit_preserved"
		return res
	}

	if origLen == 0 {
		res.Reason = "empty_input"
		return res
	}

	if mode == ModeOff {
		res.Reason = "compression_off"
		return res
	}

	// Memory safety guard: refuse inputs exceeding max bound
	if origLen > MaxProcessableBytes {
		res.Reason = "input_exceeds_memory_bound"
		return res
	}

	// Binary check: do not touch binary data
	if bytes.IndexByte(input, 0) != -1 || !utf8.Valid(input) {
		res.Reason = "binary_or_invalid_utf8"
		return res
	}

	// Output that does not end in a newline may have been cut off mid-line,
	// and CRLF output would be rewritten with stray carriage returns. In both
	// cases the raw bytes are the only faithful answer.
	if input[len(input)-1] != '\n' {
		res.Reason = "no_final_newline_preserved"
		return res
	}
	// The Flutter and Dart reporters rewrite one status line with carriage
	// returns even without a TTY; compressFlutterTest validates them exactly.
	if bytes.IndexByte(input, '\r') != -1 && !isFlutterTest(input) {
		res.Reason = "carriage_return_preserved"
		return res
	}

	// Detect format
	format, isError := detectFormat(input)
	res.Format = format

	// Safety Rule: Fail-open on error.
	// Never summarize or alter errors, stack traces, or failing test suites.
	if isError {
		res.Reason = "error_detected_fail_open_raw_preserved"
		return res
	}

	// Attempt format-specific compression
	compacted, applied, reason := applyFormatCompression(input, format)
	if !applied {
		res.Reason = reason
		return res
	}

	res.Applied = true
	res.ReducedBytes = len(compacted)
	res.Reason = reason

	if mode == ModeSafe {
		res.Output = compacted
	} else {
		// ModeObserve: calculate reductions but preserve original output untouched
		res.Reason = fmt.Sprintf("observe_mode (potential %s)", reason)
	}

	return res
}

func detectFormat(input []byte) (Format, bool) {
	if isNextBuild(input) {
		return FormatNextBuild, reNextBuildError.Match(input)
	}
	// Docker and kubectl table outputs have unambiguous headers, so they can be
	// identified early. They carry no pass/fail semantics, so isError is false.
	if isDockerPs(input) {
		return FormatDockerPs, false
	}
	if isDockerImages(input) {
		return FormatDockerImages, false
	}
	if isKubectlGetPods(input) {
		return FormatKubectlPods, false
	}
	// Package install output (npm/pnpm/yarn/bun and pip) has unambiguous
	// anchors and carries error semantics via error-shaped lines only, so it
	// is identified before the test-suite heuristics.
	if isNpmInstall(input) {
		return detectNpmInstall(input)
	}
	if isPipInstall(input) {
		return detectPipInstall(input)
	}
	if isPHPUnit(input) {
		return FormatPHPUnit, rePHPUnitFailure.Match(input)
	}
	if isFlutterTest(input) {
		return FormatFlutterTest, reFlutterTestFailure.Match(input)
	}
	if isMaven(input) {
		return FormatMaven, reMavenFailure.Match(input)
	}
	if isGradle(input) {
		return FormatGradle, reGradleFailure.Match(input)
	}

	// Verbose Go output ends with PASS, which otherwise matches Jest's generic
	// PASS header. Identify a Go test transcript before the broader Jest check.
	if reGoVerboseRun.Match(input) {
		if reGoTestFail.Match(input) {
			return FormatGoTest, true
		}
		return FormatGoTest, false
	}

	// 1. Jest must precede go test because both use a leading FAIL marker.
	if isJest(input) {
		if reJestFail.Match(input) || reJestError.Match(input) {
			return FormatJest, true
		}
		if reJestSuites.Match(input) && reJestTests.Match(input) {
			return FormatJest, false
		}
		return FormatJest, true
	}

	// 2. Go test
	if reGoTestFail.Match(input) {
		return FormatGoTest, true // Error detected! Preserve raw!
	}
	if bytes.Contains(input, []byte("PASS\n")) || reGoTestPass.Match(input) || reGoTestNoTest.Match(input) {
		return FormatGoTest, false
	}

	// 3. Pytest
	if isPytest(input) {
		// Recognition, not a danger-word list: any unrecognised line fails open.
		return FormatPytest, !isPlainPytestPass(input)
	}

	// 4. Vitest
	if isVitest(input) {
		if reVitestError.Match(input) {
			return FormatVitest, true // Error detected! Fail open!
		}
		if reVitestTests.Match(input) {
			return FormatVitest, false
		}
		return FormatVitest, true
	}

	// 5. Playwright
	if isPlaywright(input) {
		if rePlaywrightError.Match(input) {
			return FormatPlaywright, true // Error detected! Fail open!
		}
		if rePlaywrightSummary.Match(input) {
			return FormatPlaywright, false
		}
		return FormatPlaywright, true
	}

	// 6. Cypress
	if isCypress(input) {
		return FormatCypress, false
	}

	// Build errors: cargo build and dotnet build carry compiler diagnostics
	// that the cargo-test (step 7) and TSC heuristics below would otherwise
	// claim (a rustc "error[E...]:" header reads as a cargo-test error; an
	// MSBuild "file(line,col): error CODE:" reads as cargo-test via the "FAILED"
	// marker). Each detector requires a build-specific fingerprint and cargo
	// build excludes any cargo-test transcript, so a real test run never
	// matches. go build is detected later (after the linters) because its bare
	// "file.go:line:col:" grammar overlaps golangci-lint and go vet, which must
	// win. isError is false: a non-zero exit is preserved raw by CompressExit
	// before this format compacts.
	if isCargoBuild(input) {
		return FormatCargoBuild, false
	}
	if isDotnetBuild(input) {
		return FormatDotnetBuild, false
	}

	// 7. Cargo test
	if isCargoTest(input) {
		if reCargoTestError.Match(input) || !cargoTestLinesRecognised(input) {
			return FormatCargoTest, true // Error or unrecognised line! Fail open!
		}
		if reCargoTestSummary.Match(input) {
			return FormatCargoTest, false
		}
		return FormatCargoTest, true
	}

	// Linters (eslint, ruff, mypy, golangci-lint) carry file:line:col grammars
	// that the generic TSC and ripgrep heuristics below would otherwise claim.
	// Each detector requires a tool-specific fingerprint (a summary line plus a
	// rule-coded violation, or the "(linter)" suffix golangci stamps on every
	// row), so a real tsc, cargo, or search transcript never matches here. The
	// compressors group violations by file and fail open on any unrecognised
	// line, keeping every error and warning count. isError is false: a non-zero
	// exit is preserved raw by CompressExit before this format ever compacts.
	if isEslint(input) {
		return FormatEslint, false
	}
	if isRuff(input) {
		return FormatRuff, false
	}
	if isMypy(input) {
		return FormatMypy, false
	}
	if isGolangciLint(input) {
		return FormatGolangciLint, false
	}

	// go build: a "file.go:line:col: message" compiler error. Detected after the
	// linters (golangci-lint stamps the same file.go:line grammar with a
	// "(linter)" suffix and must win) and guarded against go vet, whose
	// "# [package]" bracketed marker shares the shape. Checked before TSC and
	// ripgrep, which would otherwise claim the bare file:line:col lines.
	if isGoBuild(input) {
		return FormatGoBuild, false
	}

	// 8. TSC
	if isTSC(input) {
		if reTSCError.Match(input) {
			return FormatTSC, true // Error detected! Fail open!
		}
		if reTSCPass.Match(input) {
			return FormatTSC, false
		}
		return FormatTSC, true
	}

	// 9. Git status
	if reGitStatusHead.Match(input) {
		return FormatGitStatus, false
	}

	// Git diff and git show carry a unified diff. A show transcript begins with
	// a commit line, so it is checked before git log (which also starts with
	// "commit"). Diffs carry no pass/fail semantics, so isError is false; the
	// top-level guards already preserve CRLF and non-newline-terminated output.
	// The compressors themselves fail open on any danger word or secret line
	// (git_diff.go: gitDiffUnsafe) and verify every change line survives.
	if isGitShow(input) {
		return FormatGitShow, false
	}
	if isGitDiff(input) {
		return FormatGitDiff, false
	}
	// git stash list: every line is a "stash@{N}: ..." entry.
	if isGitStashList(input) {
		return FormatGitStash, false
	}

	// 10. Git log
	if reGitLogCommit.Match(input) {
		return FormatGitLog, false
	}

	// gh CLI table outputs (pr view / pr checks / run list). gh exits non-zero
	// when a check or run fails, which CompressExit already preserves raw, so
	// a compacted table is only produced for a zero/unknown exit.
	if isGhPrView(input) {
		return FormatGhPr, false
	}
	if isGhPrChecks(input) {
		return FormatGhChecks, false
	}
	if isGhRunList(input) {
		return FormatGhRunList, false
	}
	if isGhRunLog(input) {
		return FormatGhRunLog, false
	}

	// 11. Repetitive logs, only after test suites, git status and git log had their
	// say: a failing test run or a git log with repeated lines must stay raw.
	// Timestamps can look like search hits, so this precedes the search check.
	if isRepetitiveLog(input) {
		return FormatLogs, false
	}

	// 12. Search results (rg / grep)
	if reRipgrepMatch.Match(input) {
		return FormatSearch, false
	}

	// 13. RSpec (Ruby)
	if isRspec(input) {
		return FormatRspec, reRspecFailure.Match(input)
	}

	// 14. .NET test
	if isDotnetTest(input) {
		return FormatDotnetTest, reDotnetTestFailure.Match(input)
	}

	// 15. File listings / tree
	if bytes.Contains(input, []byte("total ")) && bytes.Contains(input, []byte("drwx")) {
		return FormatFileList, false
	}

	// 16. Linters: eslint/ruff/mypy/golangci-lint are detected earlier (before
	// TSC) so the generic heuristics do not claim them. cargo clippy shares the
	// cargo test grammar, so it stays here after the test-suite checks.
	if isCargoClippy(input) {
		return FormatCargoClippy, false
	}

	// 17. Build errors (cargo build, go build, dotnet build) are detected
	// earlier, before cargo test, so the test/search/tsc heuristics do not
	// claim their compiler diagnostics.

	// 18. Package managers
	if isNpmInstall(input) {
		return FormatNpmInstall, false
	}
	if isPipInstall(input) {
		return FormatPipInstall, false
	}

	// 19. Docker
	if isDockerPs(input) {
		return FormatDockerPs, false
	}
	if isDockerImages(input) {
		return FormatDockerImages, false
	}
	if isDockerBuild(input) {
		return FormatDockerBuild, false
	}
	if isDockerPush(input) {
		return FormatDockerPush, false
	}

	// 20. Kubernetes
	if isKubectlGetPods(input) {
		return FormatKubectlPods, false
	}

	// 21. Terraform
	if isTerraformPlan(input) {
		return FormatTerraformPlan, false
	}

	// 22. Dependency trees
	if isNpmLs(input) {
		return FormatNpmLs, false
	}
	if isCargoTree(input) {
		return FormatCargoTree, false
	}
	if isGoModGraph(input) {
		return FormatGoModGraph, false
	}

	// 23. PHP ecosystem
	if isComposer(input) {
		return FormatComposer, false
	}
	if isPHPStan(input) {
		return FormatPHPStan, false
	}
	if isPsalm(input) {
		return FormatPsalm, false
	}
	if isArtisanMigrate(input) {
		return FormatArtisanMigrate, false
	}
	if isArtisanRouteList(input) {
		return FormatArtisanRoutes, false
	}

	// 24. Azure DevOps
	if isAzPipelines(input) {
		return FormatAzPipelines, false
	}

	return FormatUnknown, false
}

func applyFormatCompression(input []byte, f Format) ([]byte, bool, string) {
	switch f {
	case FormatGoTest:
		return compressGoTest(input)
	case FormatPytest:
		return compressPytest(input)
	case FormatVitest:
		return compressVitest(input)
	case FormatPlaywright:
		return compressPlaywright(input)
	case FormatCargoTest:
		return compressCargoTest(input)
	case FormatTSC:
		return compressTSC(input)
	case FormatJest:
		return compressJest(input)
	case FormatCypress:
		return compressCypress(input)
	case FormatNextBuild:
		return compressNextBuild(input)
	case FormatPHPUnit:
		return compressPHPUnit(input)
	case FormatFlutterTest:
		return compressFlutterTest(input)
	case FormatMaven:
		return compressMaven(input)
	case FormatGradle:
		return compressGradle(input)
	case FormatRspec:
		return compressRspec(input)
	case FormatDotnetTest:
		return compressDotnetTest(input)
	case FormatNpmInstall:
		return compressNpmInstall(input)
	case FormatPipInstall:
		return compressPipInstall(input)
	case FormatGitStatus:
		return compressGitStatus(input)
	case FormatGitLog:
		return compressGitLog(input)
	case FormatGitDiff:
		return compressGitDiff(input)
	case FormatGitShow:
		return compressGitShow(input)
	case FormatGitStash:
		return compressGitStashList(input)
	case FormatGhPr:
		return compressGhPrView(input)
	case FormatGhChecks:
		return compressGhPrChecks(input)
	case FormatGhRunList:
		return compressGhRunList(input)
	case FormatGhRunLog:
		return compressGhRunLog(input)
	case FormatSearch:
		return compressSearchResults(input)
	case FormatFileList:
		return compressFileList(input)
	case FormatLogs:
		return deduplicateLogs(input)
	case FormatDockerPs:
		return compressDockerPs(input)
	case FormatDockerImages:
		return compressDockerImages(input)
	case FormatDockerLogs:
		return compressDockerLogs(input)
	case FormatKubectlPods:
		return compressKubectlGetPods(input)
	case FormatKubectlLogs:
		return compressKubectlLogs(input)

	// Linters
	case FormatEslint:
		return compressEslint(input)
	case FormatRuff:
		return compressRuff(input)
	case FormatMypy:
		return compressMypy(input)
	case FormatGolangciLint:
		return compressGolangciLint(input)
	case FormatCargoClippy:
		return compressCargoClippy(input)

	// Build errors
	case FormatCargoBuild:
		return compressCargoBuild(input)
	case FormatGoBuild:
		return compressGoBuild(input)
	case FormatDotnetBuild:
		return compressDotnetBuild(input)

	// Docker build/push
	case FormatDockerBuild:
		return compressDockerBuild(input)
	case FormatDockerPush:
		return compressDockerPush(input)

	// Terraform
	case FormatTerraformPlan:
		return compressTerraformPlan(input)

	// Dependency trees
	case FormatNpmLs:
		return compressNpmLs(input)
	case FormatCargoTree:
		return compressCargoTree(input)
	case FormatGoModGraph:
		return compressGoModGraph(input)

	// PHP ecosystem
	case FormatComposer:
		return compressComposer(input)
	case FormatPHPStan:
		return compressPHPStan(input)
	case FormatPsalm:
		return compressPsalm(input)
	case FormatArtisanMigrate:
		return compressArtisanMigrate(input)
	case FormatArtisanRoutes:
		return compressArtisanRouteList(input)

	// Azure DevOps
	case FormatAzPipelines:
		return compressAzPipelines(input)

	default:
		return input, false, "unrecognized_format"
	}
}

// compressGoTest summarises a suite only when every non-empty line is an
// "ok" package line or a "? <pkg> [no test files]" line. Any other line
// (warning, skip, fail text the detector missed) keeps the original output.
// The summary keeps the pass and no-test counts, every package name (the
// module prefix shared by all of them is stated once) and the coverage
// figure when present; it drops only timings and "(cached)".
// If that is not strictly shorter than the input, the input is returned.
func compressGoTest(input []byte) ([]byte, bool, string) {
	if reGoVerboseRun.Match(input) {
		return parseGoVerbose(input)
	}

	type pkg struct {
		name    string
		cover   string
		noTests bool
	}
	var pkgs []pkg
	var okCount, noTestCount int
	for _, line := range strings.Split(string(input), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if m := reGoTestPass.FindStringSubmatch(line); m != nil {
			pkgs = append(pkgs, pkg{
				name:  m[1],
				cover: strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(m[3]), "of statements")),
			})
			okCount++
		} else if m := reGoTestNoTest.FindStringSubmatch(line); m != nil {
			pkgs = append(pkgs, pkg{
				name:    m[1],
				noTests: true,
			})
			noTestCount++
		} else if m := reGoTestCoverNoFiles.FindStringSubmatch(line); m != nil {
			pkgs = append(pkgs, pkg{
				name:    m[1],
				noTests: true,
			})
			noTestCount++
		} else {
			return input, false, "go_test_preserved_non_pass_lines"
		}
	}
	if len(pkgs) < 2 {
		return input, false, "go_test_too_few_packages_to_compress"
	}
	names := make([]string, len(pkgs))
	for i, p := range pkgs {
		names[i] = p.name
	}
	prefix := commonPathPrefix(names)

	var b strings.Builder
	if noTestCount > 0 {
		fmt.Fprintf(&b, "Go test: PASS (%d packages ok, %d without tests)", okCount, noTestCount)
	} else {
		fmt.Fprintf(&b, "Go test: PASS (%d packages ok)", okCount)
	}
	if prefix != "" {
		fmt.Fprintf(&b, " under %s", prefix)
	}
	b.WriteString("\n")
	for _, p := range pkgs {
		rel := strings.TrimPrefix(p.name, prefix)
		if p.noTests {
			b.WriteString("? " + rel + " [no test files]\n")
		} else {
			b.WriteString("ok " + rel)
			if p.cover != "" {
				b.WriteString(" " + p.cover)
			}
			b.WriteString("\n")
		}
	}
	out := b.String()
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	var reason string
	if noTestCount > 0 {
		reason = fmt.Sprintf("compressed %d packages (%d ok, %d without tests) into summary", len(pkgs), okCount, noTestCount)
	} else {
		reason = fmt.Sprintf("compressed %d passing packages into summary", len(pkgs))
	}
	return []byte(out), true, reason
}

// commonPathPrefix returns the longest prefix shared by all names that ends
// at a '/' boundary (including the slash), or "" when there is none.
func commonPathPrefix(names []string) string {
	prefix := names[0]
	for _, n := range names[1:] {
		for !strings.HasPrefix(n, prefix) {
			prefix = prefix[:len(prefix)-1]
		}
	}
	i := strings.LastIndexByte(prefix, '/')
	if i < 0 {
		return ""
	}
	return prefix[:i+1]
}

// compressGitStatus squelches a clean working tree, and only when every
// non-empty line is the branch line, an "up to date" tracking line or the
// clean marker. Ahead, behind, diverged, detached or any other line keeps the
// original output: an agent deciding whether to push needs that line.
func compressGitStatus(input []byte) ([]byte, bool, string) {
	s := string(input)
	if !strings.Contains(s, "nothing to commit, working tree clean") {
		return input, false, "git_status_has_changes_preserved"
	}
	branch := ""
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, "On branch ") && branch == "":
			branch = line
		case strings.HasPrefix(line, "Your branch is up to date"):
		case line == "nothing to commit, working tree clean":
		default:
			return input, false, "git_status_extra_lines_preserved"
		}
	}
	if branch == "" {
		return input, false, "git_status_no_branch_preserved"
	}
	return []byte(fmt.Sprintf("%s (working tree clean)\n", branch)), true, "squelched clean working tree"
}

// compressGitLog keeps the raw log. An oneline summary drops bodies.
func compressGitLog(input []byte) ([]byte, bool, string) {
	return input, false, "git_log_preserved"
}

// compressSearchResults does not drop matches. A prefix would hide hits.
func compressSearchResults(input []byte) ([]byte, bool, string) {
	return input, false, "search_results_preserved"
}

// compressFileList does not drop names. A prefix would hide entries.
func compressFileList(input []byte) ([]byte, bool, string) {
	return input, false, "file_list_preserved"
}

// isRepetitiveLog checks if successive lines have high duplicate frequency.
func isRepetitiveLog(input []byte) bool {
	lines := strings.Split(string(input), "\n")
	if len(lines) < 10 {
		return false
	}
	repeats := 0
	for i := 1; i < len(lines); i++ {
		if lines[i] != "" && lines[i] == lines[i-1] {
			repeats++
		}
	}
	return repeats >= 4
}

// deduplicateLogs collapses repeated consecutive log lines into "[repeated N times]".
func deduplicateLogs(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")
	if len(lines) == 0 {
		return input, false, "empty"
	}
	var out []string
	curLine := lines[0]
	count := 1

	for i := 1; i < len(lines); i++ {
		if lines[i] == curLine && curLine != "" {
			count++
		} else {
			if count > 1 {
				out = append(out, curLine, fmt.Sprintf("  ↳ [repeated %d times]", count-1))
			} else {
				out = append(out, curLine)
			}
			curLine = lines[i]
			count = 1
		}
	}
	if count > 1 {
		out = append(out, curLine, fmt.Sprintf("  ↳ [repeated %d times]", count-1))
	} else if curLine != "" {
		out = append(out, curLine)
	}

	res := strings.Join(out, "\n") + "\n"
	if len(res) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(res), true, "deduplicated consecutive log lines"
}

func isPytest(input []byte) bool {
	return rePytestSummary.Match(input) ||
		rePytestHeader.Match(input) ||
		rePytestFailSummary.Match(input)
}

// isPlainPytestPass reports whether every line of input is a recognised plain
// pytest line and exactly one line is a passing summary. A failure, a warning,
// a traceback or any other unrecognised line, injected or not, makes it false.
func isPlainPytestPass(input []byte) bool {
	summaries := 0
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		switch {
		case rePytestSummary.MatchString(line):
			summaries++
		case !rePytestPlainLine.MatchString(line):
			return false
		}
	}
	return summaries == 1
}

func isVitest(input []byte) bool {
	return reVitestTests.Match(input) ||
		reVitestFiles.Match(input) ||
		reVitestHeader.Match(input) ||
		reVitestFail.Match(input)
}

// compressPytest compacts clean pytest output to a one-line summary:
// Pytest: PASS (N passed[ , M skipped] in <time>)
// If not strictly shorter than raw input, the input is returned.
func compressPytest(input []byte) ([]byte, bool, string) {
	if !isPlainPytestPass(input) {
		return input, false, "pytest_unrecognised_line_preserved"
	}
	m := rePytestSummary.FindSubmatch(input)
	if m == nil {
		return input, false, "pytest_missing_summary"
	}
	skippedCount := string(m[1])
	if skippedCount == "" {
		skippedCount = string(m[3])
	}
	passedCount := string(m[2])
	timeStr := strings.TrimSpace(string(m[4]))

	var out string
	if skippedCount != "" {
		out = fmt.Sprintf("Pytest: PASS (%s passed, %s skipped in %s)\n", passedCount, skippedCount, timeStr)
	} else {
		out = fmt.Sprintf("Pytest: PASS (%s passed in %s)\n", passedCount, timeStr)
	}

	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	var reason string
	if skippedCount != "" {
		reason = fmt.Sprintf("compressed pytest summary (%s passed, %s skipped)", passedCount, skippedCount)
	} else {
		reason = fmt.Sprintf("compressed pytest summary (%s passed)", passedCount)
	}
	return []byte(out), true, reason
}

// compressVitest compacts clean vitest output to a one-line summary:
// Vitest: PASS (N passed across M files[ in <time>])
// If not strictly shorter than raw input, the input is returned.
func compressVitest(input []byte) ([]byte, bool, string) {
	if !vitestLinesRecognized(input) {
		return input, false, "vitest_unrecognized_line_preserved"
	}
	mt := reVitestTests.FindSubmatch(input)
	if mt == nil {
		return input, false, "vitest_missing_tests_summary"
	}
	nPassed := string(mt[1])

	mFiles := "1"
	if mf := reVitestFiles.FindSubmatch(input); mf != nil {
		mFiles = string(mf[1])
	}

	var timeStr string
	if md := reVitestDuration.FindSubmatch(input); md != nil {
		timeStr = strings.TrimSpace(string(md[1]))
	}

	var out string
	if timeStr != "" {
		out = fmt.Sprintf("Vitest: PASS (%s passed across %s files in %s)\n", nPassed, mFiles, timeStr)
	} else {
		out = fmt.Sprintf("Vitest: PASS (%s passed across %s files)\n", nPassed, mFiles)
	}

	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed vitest summary (%s passed across %s files)", nPassed, mFiles)
}

// vitestLinesRecognized reports whether every line of input is a plain vitest
// success line. A single unrecognised line makes the transcript unsafe to compact.
func vitestLinesRecognized(input []byte) bool {
	for _, line := range bytes.Split(input, []byte("\n")) {
		if !reVitestPlainLine.Match(line) {
			return false
		}
	}
	return true
}

func isPlaywright(input []byte) bool {
	return rePlaywrightHeader.Match(input) ||
		(rePlaywrightSummary.Match(input) && (bytes.Contains(input, []byte("worker")) || bytes.Contains(input, []byte("Running"))))
}

// The only line shapes a clean plain Playwright run prints. Recognition is the
// rule: free text is accepted only as a test title inside a passed-test line,
// and nothing may follow the closing duration. Any other line, including an
// injected error, warning or FAIL, keeps the whole transcript raw.
var (
	rePlaywrightBlankLine   = regexp.MustCompile(`^\s*$`)
	rePlaywrightRunHeader   = regexp.MustCompile(`^Running \d+ tests? using \d+ workers?$`)
	rePlaywrightPassedTest  = regexp.MustCompile(`^\s*✓\s+\d+\s+(?:\[[^\]\s]+\]\s+›\s+)?\S+(?:\s+›\s+.+?)?\s+\(\d+(?:\.\d+)?(?:ms|s|m)\)$`)
	rePlaywrightSummaryLine = regexp.MustCompile(`^\s*\d+\s+passed\s+\(\d+(?:\.\d+)?(?:ms|s|m)\)$`)
)

// playwrightLinesRecognised reports whether every line of input is part of the
// plain Playwright success grammar, with exactly one run header and one summary.
func playwrightLinesRecognised(input []byte) bool {
	headers, summaries := 0, 0
	for _, line := range bytes.Split(input, []byte("\n")) {
		switch {
		case rePlaywrightBlankLine.Match(line), rePlaywrightPassedTest.Match(line):
		case rePlaywrightRunHeader.Match(line):
			headers++
		case rePlaywrightSummaryLine.Match(line):
			summaries++
		default:
			return false
		}
	}
	return headers == 1 && summaries == 1
}

// compressPlaywright compacts clean playwright output to a one-line summary:
// Playwright: PASS (N passed in <time>)
// If not strictly shorter than raw input, the input is returned.
func compressPlaywright(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, nil) {
		return input, false, "playwright_danger_word_preserved"
	}
	if !playwrightLinesRecognised(input) {
		return input, false, "playwright_unrecognised_line_preserved"
	}
	m := rePlaywrightSummary.FindSubmatch(input)
	if m == nil {
		return input, false, "playwright_missing_summary"
	}
	passedCount := string(m[1])
	timeStr := strings.TrimSpace(string(m[2]))

	out := fmt.Sprintf("Playwright: PASS (%s passed in %s)\n", passedCount, timeStr)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed playwright summary (%s passed in %s)", passedCount, timeStr)
}

func isCargoTest(input []byte) bool {
	return reCargoTestSummary.Match(input) ||
		reCargoTestHeader.Match(input) ||
		bytes.Contains(input, []byte("test result:")) ||
		reCargoTestError.Match(input)
}

// cargoTestLinePatterns are the only line shapes a clean cargo test run emits.
// Recognition is the rule: a line that matches none of them (a compiler
// diagnostic, a panic, a failure marker, an injected warning) keeps the whole
// transcript raw, whatever its words are.
var cargoTestLinePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*Compiling \S+ v\S+ \(.*\)$`),
	regexp.MustCompile(`^\s*Finished .+ profile \[[^\]]+\] target\(s\) in \S+$`),
	regexp.MustCompile(`^\s*Running (?:unittests )?\S+ \(.*\)$`),
	regexp.MustCompile(`^\s*Doc-tests \S+$`),
	regexp.MustCompile(`^running \d+ tests?$`),
	regexp.MustCompile(`^test \S.* \.\.\. ok$`),
	regexp.MustCompile(`^test result: ok\. \d+ passed; 0 failed; \d+ ignored; \d+ measured; \d+ filtered out; finished in \S+$`),
}

// cargoTestLinesRecognised reports whether every non-empty line of input is a
// plain cargo test success line. Empty lines are structural and always allowed.
func cargoTestLinesRecognised(input []byte) bool {
	for _, line := range bytes.Split(input, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		// Free text carrying an error word is never a plain success line, even
		// when its shape fits a pattern (e.g. "Running unittests x (error: boom)").
		if bytes.Contains(bytes.ToLower(line), []byte("error")) || !matchesAnyCargoTestLine(line) {
			return false
		}
	}
	return true
}

func matchesAnyCargoTestLine(line []byte) bool {
	for _, re := range cargoTestLinePatterns {
		if re.Match(line) {
			return true
		}
	}
	return false
}

// compressCargoTest compacts clean cargo test output to a one-line summary:
// Cargo test: PASS (N passed in <time>)
// If not strictly shorter than raw input, the input is returned.
func compressCargoTest(input []byte) ([]byte, bool, string) {
	if !cargoTestLinesRecognised(input) {
		return input, false, "cargo_test_unrecognised_line_preserved"
	}
	matches := reCargoTestSummary.FindAllSubmatch(input, -1)
	if len(matches) == 0 {
		return input, false, "cargo_test_missing_summary"
	}
	var totalPassed int
	var timeStr string
	for _, m := range matches {
		var p int
		fmt.Sscanf(string(m[1]), "%d", &p)
		totalPassed += p
		if timeStr == "" || p > 0 {
			timeStr = strings.TrimSpace(string(m[2]))
		}
	}

	out := fmt.Sprintf("Cargo test: PASS (%d passed in %s)\n", totalPassed, timeStr)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed cargo test summary (%d passed in %s)", totalPassed, timeStr)
}

func isTSC(input []byte) bool {
	return reTSCPass.Match(input) || reTSCError.Match(input) || bytes.Contains(input, []byte("error TS"))
}

func isJest(input []byte) bool {
	return reJestHeader.Match(input) || reJestSuites.Match(input) || reJestTests.Match(input) || reJestFail.Match(input)
}

// compressJest compacts a clean Jest summary while retaining skipped and todo
// test counts. It requires both suite and test summaries to avoid treating
// incomplete or otherwise unrecognized output as a successful run.
func compressJest(input []byte) ([]byte, bool, string) {
	if !isKnownJestTranscript(input) {
		return input, false, "jest_unknown_output_preserved"
	}
	suites := reJestSuites.FindSubmatch(input)
	tests := reJestTests.FindSubmatch(input)
	if suites == nil || tests == nil {
		return input, false, "jest_missing_summary"
	}
	if len(suites) < 4 || len(tests) < 5 {
		return input, false, "jest_invalid_summary"
	}
	var details []string
	if tests[1] != nil {
		details = append(details, string(tests[1])+" skipped")
	}
	if tests[2] != nil {
		details = append(details, string(tests[2])+" todo")
	}
	details = append(details, string(tests[3])+" passed across "+string(suites[3])+" suites")
	if suites[1] != nil {
		unit := "suites"
		if string(suites[1]) == "1" {
			unit = "suite"
		}
		details = append(details, string(suites[1])+" "+unit+" skipped")
	}
	out := fmt.Sprintf("Jest: PASS (%s)\n", strings.Join(details, ", "))
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, fmt.Sprintf("compressed jest summary (%s tests across %s suites)", tests[3], suites[3])
}

func isKnownJestTranscript(input []byte) bool {
	for _, line := range strings.Split(strings.TrimSuffix(string(input), "\n"), "\n") {
		if strings.TrimSpace(line) == "" || reJestPass.MatchString(line) ||
			reJestSuites.MatchString(line) || reJestTests.MatchString(line) ||
			reJestSnapshots.MatchString(line) || reJestTime.MatchString(line) ||
			line == "Ran all test suites." {
			continue
		}
		return false
	}
	return true
}

// compressTSC compacts clean tsc output to a one-line summary:
// TSC: PASS (0 errors)
// If not strictly shorter than raw input, the input is returned.
func compressTSC(input []byte) ([]byte, bool, string) {
	if linesHaveDangerWord(input, reTSCSuccessLine) {
		return input, false, "tsc_danger_word_preserved"
	}
	if !tscLinesRecognized(input) {
		return input, false, "tsc_unrecognized_line_preserved"
	}
	out := "TSC: PASS (0 errors)\n"
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(out), true, "compressed tsc summary (0 errors)"
}

// tscLinesRecognized reports whether every line of input is part of the plain
// tsc success grammar and the run ends with its 0-errors summary line. A single
// unrecognised line makes the transcript unsafe to compact.
func tscLinesRecognized(input []byte) bool {
	summary := false
	for _, line := range bytes.Split(input, []byte("\n")) {
		if !reTSCPlainLine.Match(line) {
			return false
		}
		if reTSCSuccessLine.Match(line) {
			summary = true
		}
	}
	return summary
}
