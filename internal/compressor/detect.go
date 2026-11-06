// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "bytes"

// detectFormat identifies the format of input and whether the transcript shows an
// error. It tries the test runners, build tools and linters first, in a fixed
// order, then hands the rest to detectFormatTail.
func detectFormat(input []byte) (Format, bool) {
	// `go test -json` events carry the words of every other runner in their
	// Output strings, so the stream is claimed before any heuristic reads it.
	if isGoJSON(input) {
		return FormatGoTest, bytes.Contains(input, []byte(`"Action":"fail"`))
	}
	// A whole default-format git log is claimed first: its bodies may quote test
	// output, and the strict parse (git_log.go) lets nothing else through.
	if isGitLog(input) {
		return FormatGitLog, false
	}
	// A unified diff is claimed before the test-runner heuristics too: they match
	// loose words ("warning:", "FAILED") that source code in a diff quotes. The
	// diff gate (git_diff.go: gitDiffUnsafe) keeps a diff raw when such a word
	// sits on a changed line, a header or a stray line.
	if isLockfileDiff(input) {
		return FormatLockfileDiff, false
	}
	if isGitShow(input) {
		return FormatGitShow, false
	}
	if isGitDiff(input) {
		return FormatGitDiff, false
	}
	if isNx(input) {
		return FormatNx, false
	}
	if isTurbo(input) {
		return FormatTurbo, false
	}
	if isBazel(input) {
		return FormatBazel, false
	}
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
	if isNpmAudit(input) {
		return FormatNpmAudit, false
	}
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

	// Package-directory `go test -cover` prints a bare PASS line, which Jest's
	// loose header pattern also matches across the newline. An `ok` line and no
	// Jest summary identify it as Go.
	if reGoTestPass.Match(input) && bytes.HasPrefix(input, []byte("PASS\n")) &&
		!reJestSuites.Match(input) && !reJestTests.Match(input) && !reGoTestFail.Match(input) {
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
	if isRubocop(input) {
		return FormatRubocop, false
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
	return detectFormatTail(input)
}
