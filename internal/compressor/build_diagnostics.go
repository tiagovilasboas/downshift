// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
)

// buildDiagnostics accumulates compiler diagnostics grouped by file in a
// stable, deterministic order. Unlike violationGroups (used by the linters,
// which keep only per-file counts and rule names), it preserves the full
// diagnostic message line of every error and warning so a build summary never
// drops the fact an agent needs to act on. Shared by the build compressors
// (cargo build, go build, dotnet build).
type buildDiagnostics struct {
	order  []string
	byFile map[string]*fileDiagnostics
}

type fileDiagnostics struct {
	errors   int
	warnings int
	messages []string
}

func newBuildDiagnostics() *buildDiagnostics {
	return &buildDiagnostics{byFile: make(map[string]*fileDiagnostics)}
}

func (b *buildDiagnostics) ensure(file string) *fileDiagnostics {
	fd, ok := b.byFile[file]
	if !ok {
		fd = &fileDiagnostics{}
		b.byFile[file] = fd
		b.order = append(b.order, file)
	}
	return fd
}

// add records one diagnostic for a file. message is the full header line and
// is preserved verbatim in render output so no detail is lost.
func (b *buildDiagnostics) add(file string, isError bool, message string) {
	fd := b.ensure(file)
	if isError {
		fd.errors++
	} else {
		fd.warnings++
	}
	if message != "" {
		fd.messages = append(fd.messages, message)
	}
}

func (b *buildDiagnostics) fileCount() int {
	n := 0
	for _, f := range b.order {
		fd := b.byFile[f]
		if fd.errors > 0 || fd.warnings > 0 {
			n++
		}
	}
	return n
}

// render produces the RTK build format: a summary header with the total
// error/warning counts, then one block per file with the diagnostics it
// carries, in first-seen order.
//
//	[ ] cargo build: 3 errors, 2 warnings
//	src/lib.rs:
//	  error[E0308]: mismatched types
//	  warning: unused variable: `x`
//
// The leading "[ ]" marks a non-passing build at a glance, mirroring RTK.
func (b *buildDiagnostics) render(tool string) []byte {
	totalErr, totalWarn := 0, 0
	for _, f := range b.order {
		fd := b.byFile[f]
		totalErr += fd.errors
		totalWarn += fd.warnings
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "[ ] %s: %s, %s\n",
		tool,
		pluralCount(totalErr, "error", "errors"),
		pluralCount(totalWarn, "warning", "warnings"),
	)
	for _, file := range b.order {
		fd := b.byFile[file]
		if fd.errors == 0 && fd.warnings == 0 {
			continue
		}
		buf.WriteString(file)
		buf.WriteString(":\n")
		for _, msg := range fd.messages {
			buf.WriteString("  ")
			buf.WriteString(strings.TrimSpace(msg))
			buf.WriteString("\n")
		}
	}
	return []byte(buf.String())
}
