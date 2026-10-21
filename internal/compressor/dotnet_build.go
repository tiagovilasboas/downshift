// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// dotnet build (MSBuild) output. A build prints a restore banner, per-project
// "Determining projects to restore..." noise, then diagnostics in MSBuild
// canonical form and a final summary:
//
//	Determining projects to restore...
//	  Restored /path/App.csproj (in 1.2 sec).
//	  App -> /path/bin/Debug/net8.0/App.dll
//	/path/Program.cs(12,5): error CS0103: The name 'x' does not exist [/path/App.csproj]
//	/path/Program.cs(20,9): warning CS0168: The variable 'y' is declared but never used [/path/App.csproj]
//	Build FAILED.
//	    2 Warning(s)
//	    1 Error(s)
//
// The compressor keeps only the "file(line,col): error|warning CODE: message"
// diagnostics, grouped by file (dropping the trailing "[project.csproj]"),
// and strips the restore banner, the "->" output lines and the summary. Any
// line outside this known grammar makes the whole transcript pass through
// unchanged, so an unexpected shape never loses a fact.
var (
	// "/path/Program.cs(12,5): error CS0103: message [/path/App.csproj]"
	// The optional trailing " [project]" is excluded from the message.
	reDotnetBuildDiag = regexp.MustCompile(`^(?:\s*)(\S.*?)\((\d+),(\d+)\):\s+(error|warning)\s+([A-Za-z]+\d+):\s+(.*?)(?:\s+\[[^\]]+\])?\s*$`)
	// Multiline guard: at least one MSBuild diagnostic anywhere in the input.
	reDotnetBuildDiagM = regexp.MustCompile(`(?m)^\s*\S.*?\(\d+,\d+\):\s+(?:error|warning)\s+[A-Za-z]+\d+:\s+`)
	// Restore / status noise.
	reDotnetBuildStatus = regexp.MustCompile(`^\s*(?:Determining projects to restore\.\.\.|Restored\b|Restore\b|MSBuild version\b|Microsoft \(R\) Build Engine\b|Copyright\b)`)
	// "  App -> /path/bin/Debug/net8.0/App.dll" output artifact line.
	reDotnetBuildOutput = regexp.MustCompile(`^\s+\S.*\s->\s+\S`)
	// "Build succeeded.", "Build FAILED.", "    2 Warning(s)", "    1 Error(s)",
	// "Time Elapsed 00:00:01.23".
	reDotnetBuildSummary = regexp.MustCompile(`^\s*(?:Build succeeded\.|Build FAILED\.|\d+\s+Warning\(s\)|\d+\s+Error\(s\)|Time Elapsed\b)`)
)

// isDotnetBuild returns true when the output carries at least one MSBuild
// "file(line,col): error|warning CODE:" diagnostic: the fingerprint of a
// dotnet build. It is distinct from dotnet test (see dotnet.go), which uses a
// "Passed!/Failed!" summary line rather than MSBuild canonical diagnostics.
func isDotnetBuild(input []byte) bool {
	return reDotnetBuildDiagM.Match(input)
}

// compressDotnetBuild groups dotnet build errors and warnings by file,
// preserving each diagnostic (code and message) while stripping the restore
// banner, the "->" output artifacts and the final summary. An input with any
// unrecognized line, or no diagnostics to group, is returned byte-for-byte.
func compressDotnetBuild(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00") {
		return input, false, "dotnet_build_invalid_or_incomplete_input"
	}
	if !reDotnetBuildDiagM.Match(input) {
		return input, false, "dotnet_build_no_diagnostics"
	}

	lines := strings.Split(strings.TrimSuffix(string(input), "\n"), "\n")
	groups := newBuildDiagnostics()
	for _, line := range lines {
		if m := reDotnetBuildDiag.FindStringSubmatch(line); m != nil {
			file := m[1]
			isError := m[4] == "error"
			// Preserve a normalized "file(line,col): sev CODE: message".
			msg := fmt.Sprintf("%s(%s,%s): %s %s: %s", m[1], m[2], m[3], m[4], m[5], m[6])
			groups.add(file, isError, msg)
			continue
		}
		if line == "" ||
			reDotnetBuildStatus.MatchString(line) ||
			reDotnetBuildOutput.MatchString(line) ||
			reDotnetBuildSummary.MatchString(line) {
			continue
		}
		// Anything else is outside the known grammar: preserve everything.
		return input, false, "dotnet_build_unrecognized_line"
	}

	if groups.fileCount() == 0 {
		return input, false, "dotnet_build_no_diagnostics_grouped"
	}

	out := groups.render("dotnet build")
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed dotnet build (%d files)", groups.fileCount())
}
