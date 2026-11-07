// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// npm/pnpm/yarn/bun install output compressor.
// These package managers produce hundreds of lines of dependency resolution,
// peer dependency warnings, and deprecation notices that LLMs don't need.
// Installed packages only, progress stripped.

var (
	// npm patterns
	reNpmAdded    = regexp.MustCompile(`(?m)^added (\d+) packages?(?:.*?in (\d+(?:\.\d+)?m?s))?`)
	reNpmUpToDate = regexp.MustCompile(`(?m)^up to date(?:.*?in (\d+(?:\.\d+)?m?s))?`)

	// pnpm patterns
	rePnpmPackages = regexp.MustCompile(`(?m)^Packages: \+(\d+)`)
	rePnpmProgress = regexp.MustCompile(`(?m)^Progress: resolved (\d+), reused (\d+), downloaded (\d+), added (\d+)(?:, done)?`)
	rePnpmDone     = regexp.MustCompile(`(?m)^Done in (\d+(?:\.\d+)?m?s)`)
	rePnpmLockfile = regexp.MustCompile(`(?m)^Lockfile is up to date`)

	// yarn patterns
	reYarnSuccess  = regexp.MustCompile(`(?m)^success Saved lockfile`)
	reYarnDone     = regexp.MustCompile(`(?m)^Done in (\d+(?:\.\d+)?m?s)`)
	reYarnResolved = regexp.MustCompile(`(?m)^Resolving packages\.\.\.`)

	// bun patterns
	reBunInstalled = regexp.MustCompile(`(?m)^\s*(\d+) packages? installed`)
	reBunDone      = regexp.MustCompile(`(?m)\[(\d+(?:\.\d+)?m?s)\]$`)

	// Error indicators (fail-open)
	reNpmError = regexp.MustCompile(`(?im)(?:^npm ERR!|^ERR!|ERESOLVE|ENOENT|EACCES|EPERM|E404|ETARGET|ENOTFOUND|code E\d{3})`)
)

// isNpmInstall detects npm/pnpm/yarn/bun install output.
func isNpmInstall(input []byte) bool {
	// npm indicators
	if reNpmAdded.Match(input) || reNpmUpToDate.Match(input) {
		return true
	}
	// pnpm indicators
	if rePnpmPackages.Match(input) || rePnpmProgress.Match(input) {
		return true
	}
	// yarn indicators
	if reYarnSuccess.Match(input) || (reYarnResolved.Match(input) && reYarnDone.Match(input)) {
		return true
	}
	// bun indicators
	if reBunInstalled.Match(input) {
		return true
	}
	return false
}

// detectNpmInstall reports the format and whether the output is error-shaped.
func detectNpmInstall(input []byte) (Format, bool) {
	if !isNpmInstall(input) {
		return FormatUnknown, false
	}
	return FormatNpmInstall, reNpmError.Match(input)
}

// compressNpmInstall compacts package manager install output to essentials.
// Only the installed-packages summary survives; progress and per-package
// lines are stripped. A warning, a deprecation notice, an error or any line
// classifyInstall does not know keeps the run raw; audit findings are kept
// verbatim.
func compressNpmInstall(input []byte) ([]byte, bool, string) {
	// Fail-open on errors
	if reNpmError.Match(input) {
		return input, false, "npm_error_preserved"
	}

	findings, known := classifyInstall(input)
	if !known {
		return input, false, "npm_unrecognised_line_preserved"
	}

	var summary strings.Builder
	hasContent := false

	// Try npm format
	if m := reNpmAdded.FindSubmatch(input); m != nil {
		hasContent = true
		summary.WriteString(fmt.Sprintf("[x] npm: added %s %s", string(m[1]), packageNoun(string(m[1]))))
		if len(m) > 2 && len(m[2]) > 0 {
			summary.WriteString(fmt.Sprintf(" in %s", string(m[2])))
		}
		summary.WriteString("\n")
	} else if reNpmUpToDate.Match(input) {
		hasContent = true
		if m := reNpmUpToDate.FindSubmatch(input); len(m) > 1 && len(m[1]) > 0 {
			summary.WriteString(fmt.Sprintf("[x] npm: up to date in %s\n", string(m[1])))
		} else {
			summary.WriteString("[x] npm: up to date\n")
		}
	}

	// Try pnpm format
	if m := rePnpmPackages.FindSubmatch(input); m != nil {
		hasContent = true
		summary.WriteString(fmt.Sprintf("[x] pnpm: +%s %s", string(m[1]), packageNoun(string(m[1]))))
		if dm := rePnpmDone.FindSubmatch(input); dm != nil {
			summary.WriteString(fmt.Sprintf(" in %s", string(dm[1])))
		}
		summary.WriteString("\n")
	} else if rePnpmLockfile.Match(input) && rePnpmDone.Match(input) {
		hasContent = true
		if dm := rePnpmDone.FindSubmatch(input); dm != nil {
			summary.WriteString(fmt.Sprintf("[x] pnpm: up to date in %s\n", string(dm[1])))
		}
	}

	// Try yarn format
	if reYarnSuccess.Match(input) {
		hasContent = true
		summary.WriteString("[x] yarn: installed")
		if m := reYarnDone.FindSubmatch(input); m != nil {
			summary.WriteString(fmt.Sprintf(" in %s", string(m[1])))
		}
		summary.WriteString("\n")
	}

	// Try bun format
	if m := reBunInstalled.FindSubmatch(input); m != nil {
		hasContent = true
		summary.WriteString(fmt.Sprintf("[x] bun: %s %s installed", string(m[1]), packageNoun(string(m[1]))))
		if dm := reBunDone.FindSubmatch(input); dm != nil {
			summary.WriteString(fmt.Sprintf(" in %s", string(dm[1])))
		}
		summary.WriteString("\n")
	}

	// Findings (vulnerabilities the install reported) stay verbatim.
	for _, line := range findings {
		summary.WriteString(line + "\n")
	}

	if !hasContent {
		return input, false, "npm_format_not_detected"
	}

	if summary.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(summary.String()), true, "npm_install_compacted"
}

// packageNoun is "package" for a count of one and "packages" otherwise, so a
// summary reads the way the tool printed its count.
func packageNoun(count string) string {
	if count == "1" {
		return "package"
	}
	return "packages"
}
