// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

var (
	reGoTestPass   = lazyre.New(`(?m)^ok\s+([^\s]+)\s+([^\s]+)(?:\s+coverage:\s+(.+))?$`)
	reGoTestNoTest = lazyre.New(`(?m)^\?\s+([^\s]+)\s+\[no test files\]\s*$`)
	// reGoTestCoverNoFiles is the line `go test -cover` prints for a package
	// with no test files under the tab-indented form (no "ok" or "?" prefix).
	reGoTestCoverNoFiles = lazyre.New(`^\t(\S+)\t+coverage: [\d.]+% of statements$`)
	reGoTestFail         = lazyre.New(`(?m)^(?:--- FAIL:|FAIL|\s*panic:)`)
)

// compressGoTest folds a clean (exit 0) go test run. A -v transcript goes to
// parseGoVerbose; a plain run goes to foldGoTestPlain. Both keep every line
// they cannot positively account for, and fall back to the raw output when
// the result is not strictly shorter.
func compressGoTest(input []byte) ([]byte, bool, string) {
	// Our own summary fed back in: leave it alone so compaction is idempotent.
	if bytes.HasPrefix(input, []byte("Go test")) {
		return input, false, "go_test_already_folded"
	}
	if isGoJSON(input) {
		return compressGoTestJSON(input)
	}
	if reGoVerboseRun.Match(input) {
		if countGoVerbosePackages(input) > 1 {
			return parseGoVerboseMulti(input)
		}
		return parseGoVerbose(input)
	}
	return foldGoTestPlain(input)
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
