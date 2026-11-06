// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Azure DevOps CLI (az pipelines) output compressor.
// Handles: az pipelines run list, az pipelines build show, etc.

var (
	// az pipelines patterns - JSON array output
	reAzPipelinesJSON    = regexp.MustCompile(`^\s*\[\s*\{`)
	reAzPipelinesBuildID = regexp.MustCompile(`"buildNumber":\s*"([^"]+)"`)
	reAzPipelinesStatus  = regexp.MustCompile(`"status":\s*"([^"]+)"`)
	reAzPipelinesResult  = regexp.MustCompile(`"result":\s*"([^"]+)"`)
	reAzPipelinesBranch  = regexp.MustCompile(`"sourceBranch":\s*"refs/heads/([^"]+)"`)

	// az pipelines table output (--output table)
	reAzPipelinesTableHeader = regexp.MustCompile(`(?m)^(?:ID|Build Number|Run ID)\s+`)
)

// isAzPipelines detects az pipelines output.
func isAzPipelines(input []byte) bool {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return false
	}
	return (reAzPipelinesJSON.Match(input) && reAzPipelinesBuildID.Match(input)) ||
		reAzPipelinesTableHeader.Match(input)
}

// compressAzPipelines compacts az pipelines output.
func compressAzPipelines(input []byte) ([]byte, bool, string) {
	if len(input) > MaxProcessableBytes || !utf8.Valid(input) {
		return input, false, "az_pipelines_too_large"
	}

	// Handle JSON array output
	if reAzPipelinesJSON.Match(input) {
		return compressAzPipelinesJSON(input)
	}

	// Handle table output
	if reAzPipelinesTableHeader.Match(input) {
		return compressAzPipelinesTable(input)
	}

	return input, false, "az_pipelines_unknown_format"
}

func compressAzPipelinesJSON(input []byte) ([]byte, bool, string) {
	// Extract key fields from JSON without full parsing
	builds := bytes.Split(input, []byte("},{"))

	var out bytes.Buffer
	out.WriteString("az pipelines: ")
	out.WriteString(itoa(len(builds)))
	out.WriteString(" runs\n")

	// Summarize by status/result
	statusCounts := make(map[string]int)
	resultCounts := make(map[string]int)

	for _, build := range builds {
		if m := reAzPipelinesStatus.FindSubmatch(build); m != nil {
			statusCounts[string(m[1])]++
		}
		if m := reAzPipelinesResult.FindSubmatch(build); m != nil {
			resultCounts[string(m[1])]++
		}
	}

	// Show status breakdown
	if len(statusCounts) > 0 {
		out.WriteString("  status: ")
		var parts []string
		for status, count := range statusCounts {
			parts = append(parts, status+"("+itoa(count)+")")
		}
		out.WriteString(strings.Join(parts, ", "))
		out.WriteString("\n")
	}

	// Show result breakdown
	if len(resultCounts) > 0 {
		out.WriteString("  result: ")
		var parts []string
		for result, count := range resultCounts {
			parts = append(parts, result+"("+itoa(count)+")")
		}
		out.WriteString(strings.Join(parts, ", "))
		out.WriteString("\n")
	}

	// Show latest build details
	if len(builds) > 0 {
		latest := builds[0]
		out.WriteString("  latest: ")
		if m := reAzPipelinesBuildID.FindSubmatch(latest); m != nil {
			out.Write(m[1])
		}
		if m := reAzPipelinesBranch.FindSubmatch(latest); m != nil {
			out.WriteString(" (")
			out.Write(m[1])
			out.WriteString(")")
		}
		if m := reAzPipelinesResult.FindSubmatch(latest); m != nil {
			out.WriteString(" → ")
			out.Write(m[1])
		}
		out.WriteString("\n")
	}

	return out.Bytes(), true, "az_pipelines_json_compacted"
}

func compressAzPipelinesTable(input []byte) ([]byte, bool, string) {
	lines := strings.Split(string(input), "\n")

	var dataLines int
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "-") && !reAzPipelinesTableHeader.MatchString(line) {
			dataLines++
		}
	}

	if dataLines == 0 {
		return input, false, "az_pipelines_table_empty"
	}

	var out bytes.Buffer
	out.WriteString("az pipelines: ")
	out.WriteString(itoa(dataLines))
	out.WriteString(" runs (use --output json for details)\n")

	// Keep header + first 5 data rows
	lineCount := 0
	for _, line := range lines {
		if lineCount >= 7 { // header + separator + 5 rows
			break
		}
		if strings.TrimSpace(line) != "" {
			out.WriteString(line)
			out.WriteString("\n")
			lineCount++
		}
	}

	if dataLines > 5 {
		out.WriteString("... and ")
		out.WriteString(itoa(dataLines - 5))
		out.WriteString(" more\n")
	}

	return out.Bytes(), true, "az_pipelines_table_compacted"
}

// FormatAzPipelines constant
const FormatAzPipelines Format = "az_pipelines"
