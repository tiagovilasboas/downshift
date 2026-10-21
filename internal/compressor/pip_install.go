// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
)

// pip install output compressor.
// pip prints one "Collecting", "Downloading" and "Using cached" line per
// dependency, plus a multi-line "Installing collected packages" block. LLMs
// only need the final result.
// RTK parity: installed packages only, progress stripped.

var (
	// "Successfully installed flask-3.0.0 jinja2-3.1.2 werkzeug-3.0.1"
	rePipInstalled = regexp.MustCompile(`(?m)^Successfully installed (.+)$`)
	// "Requirement already satisfied: flask in ..." (no-op install)
	rePipAlreadySatisfied = regexp.MustCompile(`(?m)^Requirement already satisfied:`)
	// Progress / resolution noise to strip (used only for detection intent).
	rePipCollecting  = regexp.MustCompile(`(?m)^Collecting `)
	rePipDownloading = regexp.MustCompile(`(?m)^\s*Downloading `)
	// Non-fatal upgrade hint pip prints on a successful run.
	rePipUpgradeNotice = regexp.MustCompile(`(?m)^\[notice\]`)

	// Error indicators (fail-open). pip reports failures with ERROR:, a
	// traceback, or a non-resolvable dependency message.
	rePipError = regexp.MustCompile(`(?im)(?:^ERROR:|^WARNING: .*did not run successfully|Could not find a version|No matching distribution|ResolutionImpossible|Traceback \(most recent call last\))`)
)

// isPipInstall detects pip install output.
func isPipInstall(input []byte) bool {
	return rePipInstalled.Match(input) ||
		rePipAlreadySatisfied.Match(input) ||
		rePipCollecting.Match(input) ||
		rePipDownloading.Match(input)
}

// detectPipInstall reports the format and whether the output is error-shaped.
func detectPipInstall(input []byte) (Format, bool) {
	if !isPipInstall(input) {
		return FormatUnknown, false
	}
	return FormatPipInstall, rePipError.Match(input)
}

// compressPipInstall compacts pip install output to the installed-packages
// summary. Collecting/Downloading/Installing lines are stripped; the final
// "Successfully installed" list is preserved as a count. Fails open on any
// error-shaped output.
func compressPipInstall(input []byte) ([]byte, bool, string) {
	// Fail-open on errors
	if rePipError.Match(input) {
		return input, false, "pip_error_preserved"
	}

	if m := rePipInstalled.FindSubmatch(input); m != nil {
		pkgs := strings.Fields(string(m[1]))
		return []byte(fmt.Sprintf("[x] pip: installed %d packages\n", len(pkgs))), true, "pip_install_compacted"
	}

	// Nothing to install: every requirement already satisfied.
	if rePipAlreadySatisfied.Match(input) {
		return []byte("[x] pip: all requirements already satisfied\n"), true, "pip_install_compacted"
	}

	return input, false, "pip_format_not_detected"
}
