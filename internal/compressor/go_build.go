// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// go build output. A clean build prints nothing; a failing build prints
// compiler errors in "file.go:line:col: message" form, optionally preceded by
// a "# package/path" marker and module "go: downloading ..." status lines:
//
//	go: downloading github.com/foo/bar v1.2.3
//	# example.com/app/internal/core
//	internal/core/parse.go:12:5: undefined: Foo
//	internal/core/parse.go:20:2: cannot use x (untyped int constant) as string value
//
// go build does not emit warnings, so every diagnostic is an error. The
// compressor strips the "go: downloading" noise and the "# package" markers,
// groups the errors by file and preserves each message. Any line outside this
// grammar makes the whole transcript pass through unchanged.
var (
	// "internal/core/parse.go:12:5: message" — column may be absent.
	reGoBuildError = regexp.MustCompile(`^(\S+\.go):(\d+):(?:(\d+):)?\s+(.*)$`)
	// Multiline guard: at least one "file.go:line: message" compiler error
	// anywhere in the input. The per-line reGoBuildError stays single-line for
	// the compressor's line loop; this scans the whole transcript for detection.
	reGoBuildErrorM = regexp.MustCompile(`(?m)^\S+\.go:\d+:(?:\d+:)?\s+\S`)
	// go vet marks its target with a bracketed "# [package]" header that go
	// build never prints; its diagnostics otherwise share the ".go:line:col:"
	// grammar, so this marker keeps a vet transcript out of the build compressor.
	reGoVetMarker = regexp.MustCompile(`(?m)^# \[`)
	// golangci-lint stamps every violation with a trailing "(linter)" suffix.
	reGolangciSuffix = regexp.MustCompile(`(?m)^\S+:\d+:\d+:.*\([a-z0-9_-]+\)\s*$`)
	// "go: downloading github.com/foo/bar v1.2.3" and sibling module status.
	reGoBuildDownloading = regexp.MustCompile(`^go: (?:downloading|extracting|finding|found|added|upgraded|downgraded)\b`)
	// "# example.com/app/internal/core" package marker.
	reGoBuildPackage = regexp.MustCompile(`^# \S`)
)

// isGoBuild returns true when the output carries at least one
// "file.go:line: message" compiler error: the fingerprint of a failing build.
func isGoBuild(input []byte) bool {
	if reGoVetMarker.Match(input) || reGolangciSuffix.Match(input) {
		return false
	}
	return reGoBuildErrorM.Match(input)
}

// compressGoBuild groups go build errors by file, dropping the module
// "go: downloading" status and "# package" markers while preserving each
// compiler message. An input with any unrecognized line, or no errors to
// group, is returned byte-for-byte.
func compressGoBuild(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00\x1b") {
		return input, false, "go_build_invalid_or_incomplete_input"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newBuildDiagnostics()
	for _, line := range lines {
		if line == "" || reGoBuildDownloading.MatchString(line) || reGoBuildPackage.MatchString(line) {
			continue
		}
		if m := reGoBuildError.FindStringSubmatch(line); m != nil {
			// go build errors have no severity word; all are errors. The file
			// becomes the group header, so drop it from the message and keep
			// "line:col: message" to avoid repeating the path.
			file := m[1]
			msg := strings.TrimPrefix(line, file+":")
			groups.add(file, true, msg)
			continue
		}
		// A line that is not a stripped status line and not a compiler error
		// (an indented continuation, a linker message, a panic) means the
		// transcript is outside the known grammar: preserve everything.
		return input, false, "go_build_unrecognized_line"
	}

	if groups.fileCount() == 0 {
		return input, false, "go_build_no_errors_grouped"
	}

	out := groups.render("go build")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed go build (%d files)", groups.fileCount())
}
