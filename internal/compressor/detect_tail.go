// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import "bytes"

// detectFormatTail is the second stage of detectFormat: git, the gh CLI,
// repetitive logs, search results, listings, package managers, containers,
// infrastructure, PHP tools and Azure DevOps.
func detectFormatTail(input []byte) (Format, bool) {
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
	// A diff of lockfiles only is summarised to the packages that moved.
	if isLockfileDiff(input) {
		return FormatLockfileDiff, false
	}
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
	if isGhMarkdownView(input) {
		return FormatMarkdown, false
	}
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
