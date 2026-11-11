// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// pip install output compressor.
// pip prints one "Collecting", "Downloading" and "Using cached" line per
// dependency, plus a multi-line "Installing collected packages" block. LLMs
// only need the final result.
// Installed packages only, progress stripped.

var (
	// "Successfully installed flask-3.0.0 jinja2-3.1.2 werkzeug-3.0.1"
	rePipInstalled = lazyre.New(`(?m)^Successfully installed (.+)$`)
	// "Requirement already satisfied: flask in ..." (no-op install)
	rePipAlreadySatisfied = lazyre.New(`(?m)^Requirement already satisfied:`)
	// Progress / resolution noise to strip (used only for detection intent).
	rePipCollecting  = lazyre.New(`(?m)^Collecting `)
	rePipDownloading = lazyre.New(`(?m)^\s*Downloading `)

	// Findings (fail-open). pip reports failures with ERROR:, a warning with
	// WARNING:, a traceback, or a non-resolvable dependency message; the
	// summary would drop any of them.
	rePipError = lazyre.New(`(?im)(?:^ERROR:|^WARNING:|Could not find a version|No matching distribution|ResolutionImpossible|Traceback \(most recent call last\))`)
)

// pipHasFindings reports output the summary must not replace.
func pipHasFindings(input []byte) bool {
	return rePipError.Match(input) || reDangerWord.Match(input)
}

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
	return FormatPipInstall, pipHasFindings(input)
}

// compressPipInstall compacts pip install output to the installed-packages
// summary. Collecting/Downloading/Installing lines are stripped; the final
// "Successfully installed" list is preserved as a count. Fails open on any
// finding (pipHasFindings).
func compressPipInstall(input []byte) ([]byte, bool, string) {
	if pipHasFindings(input) {
		return input, false, "pip_error_preserved"
	}

	if m := rePipInstalled.FindSubmatch(input); m != nil {
		pkgs := strings.Fields(string(m[1]))
		n := strconv.Itoa(len(pkgs))
		return []byte(fmt.Sprintf("[x] pip: installed %s %s\n", n, packageNoun(n))), true, "pip_install_compacted"
	}

	// Nothing to install: every requirement already satisfied.
	if rePipAlreadySatisfied.Match(input) {
		return []byte("[x] pip: all requirements already satisfied\n"), true, "pip_install_compacted"
	}

	return input, false, "pip_format_not_detected"
}
