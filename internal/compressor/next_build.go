// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	reNextBuildHeader = regexp.MustCompile(`^▲ Next\.js\s+\S+\s*$`)
	reNextBuildTime   = regexp.MustCompile(`^✓ Compiled successfully(?: in ([0-9]+(?:\.[0-9]+)?s))?$`)
	reNextBuildPages  = regexp.MustCompile(`^✓ Generating static pages \(([0-9]+)/([0-9]+)\)$`)
	reNextBuildRoute  = regexp.MustCompile(`^[┌├└] [○●ƒλ] /\S*\s+[0-9]+(?:\.[0-9]+)? (?:B|kB|MB)\s+[0-9]+(?:\.[0-9]+)? (?:B|kB|MB)$`)
	reNextBuildShared = regexp.MustCompile(`^\+ First Load JS shared by all\s+([0-9]+(?:\.[0-9]+)? (?:B|kB|MB))$`)
	reNextBuildChunk  = regexp.MustCompile(`^  [├└] chunks/\S+\s+[0-9]+(?:\.[0-9]+)? (?:B|kB|MB)$`)
	reNextBuildOther  = regexp.MustCompile(`^  [├└] other shared chunks \(total\)\s+[0-9]+(?:\.[0-9]+)? (?:B|kB|MB)$`)
	reNextBuildError  = regexp.MustCompile(`(?i)(?:^|\b)(?:warning|error|failed|failure|exception|traceback)(?:\b|:)`)
)

func isNextBuild(input []byte) bool {
	return reNextBuildHeader.MatchString(strings.SplitN(string(input), "\n", 2)[0])
}

// compressNextBuild summarizes only a complete, recognized successful Next.js
// build transcript. Unknown lines, diagnostics, and incomplete route/chunk
// tables are preserved byte-for-byte.
func compressNextBuild(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00") {
		return input, false, "next_build_invalid_or_incomplete_input"
	}
	output := string(input)
	if reNextBuildError.MatchString(output) {
		return input, false, "next_build_diagnostic_preserved"
	}

	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	if len(lines) < 12 || !reNextBuildHeader.MatchString(lines[0]) {
		return input, false, "next_build_unrecognized_transcript"
	}

	var duration string
	phase := 0
	routeSections := 0
	routes := 0
	routesInSection := 0
	routeCounts := make(map[string]int, 2)
	currentSection := ""
	lastRouteGlyph := ""
	seenSections := make(map[string]bool, 2)
	seenRoutes := make(map[string]bool)
	sharedChunks := false
	sharedSize := ""
	otherChunks := false
	legends := 0
	for _, line := range lines[1:] {
		if line == "" {
			continue
		}
		switch {
		case reNextBuildTime.MatchString(line):
			if phase != 0 {
				return input, false, "next_build_ambiguous_phase_order"
			}
			phase = 1
			if matches := reNextBuildTime.FindStringSubmatch(line); matches != nil {
				duration = matches[1]
			}
		case line == "✓ Linting and checking validity of types":
			if phase != 1 {
				return input, false, "next_build_ambiguous_phase_order"
			}
			phase = 2
		case line == "✓ Collecting page data":
			if phase != 2 {
				return input, false, "next_build_ambiguous_phase_order"
			}
			phase = 3
		case reNextBuildPages.MatchString(line):
			matches := reNextBuildPages.FindStringSubmatch(line)
			if phase != 3 || matches[1] != matches[2] {
				return input, false, "next_build_incomplete_static_generation"
			}
			phase = 4
		case line == "✓ Finalizing page optimization":
			if phase != 4 {
				return input, false, "next_build_ambiguous_phase_order"
			}
			phase = 5
		case line == "✓ Collecting build traces":
			if phase != 5 {
				return input, false, "next_build_ambiguous_phase_order"
			}
			phase = 6
		case strings.HasPrefix(line, "Route (") && strings.HasSuffix(line, ")"):
			section := strings.TrimSuffix(strings.TrimPrefix(line, "Route ("), ")")
			if phase != 6 || (section != "app" && section != "pages") || seenSections[section] || sharedChunks {
				return input, false, "next_build_ambiguous_route_sections"
			}
			if routeSections > 0 && (routesInSection == 0 || lastRouteGlyph != "└") {
				return input, false, "next_build_incomplete_route_section"
			}
			seenSections[section] = true
			routeSections++
			currentSection = section
			routesInSection = 0
			lastRouteGlyph = ""
		case strings.HasPrefix(line, "+ First Load JS shared by all"):
			if phase != 6 || routeSections == 0 || routesInSection == 0 || lastRouteGlyph != "└" || !reNextBuildShared.MatchString(line) || sharedChunks {
				return input, false, "next_build_unrecognized_chunk_summary"
			}
			sharedChunks = true
			sharedSize = reNextBuildShared.FindStringSubmatch(line)[1]
		case strings.HasPrefix(line, "  ├ chunks/") || strings.HasPrefix(line, "  └ chunks/"):
			if !sharedChunks || otherChunks || !reNextBuildChunk.MatchString(line) {
				return input, false, "next_build_unrecognized_chunk_row"
			}
		case strings.HasPrefix(line, "  └ other shared chunks (total)"):
			if !sharedChunks || otherChunks || !reNextBuildOther.MatchString(line) {
				return input, false, "next_build_unrecognized_chunk_summary"
			}
			otherChunks = true
		case strings.HasPrefix(line, "┌ ") || strings.HasPrefix(line, "├ ") || strings.HasPrefix(line, "└ "):
			if phase != 6 || routeSections == 0 || sharedChunks || !reNextBuildRoute.MatchString(line) {
				return input, false, "next_build_unrecognized_route_row"
			}
			fields := strings.Fields(line)
			if len(fields) < 3 {
				return input, false, "next_build_unrecognized_route_row"
			}
			route := fields[2]
			if seenRoutes[route] {
				return input, false, "next_build_duplicate_route"
			}
			seenRoutes[route] = true
			routes++
			routesInSection++
			routeCounts[currentSection]++
			lastRouteGlyph = fields[0]
		case strings.HasPrefix(line, "○ (Static) ") || strings.HasPrefix(line, "○  (Static) ") || strings.HasPrefix(line, "ƒ (Dynamic) ") || strings.HasPrefix(line, "ƒ  (Dynamic) ") || strings.HasPrefix(line, "● (SSG) ") || strings.HasPrefix(line, "●  (SSG) ") || strings.HasPrefix(line, "λ (Server) ") || strings.HasPrefix(line, "λ  (Server) "):
			if phase != 6 || !sharedChunks || !otherChunks {
				return input, false, "next_build_incomplete_chunk_table"
			}
			legends++
		default:
			return input, false, "next_build_unrecognized_line"
		}
	}
	if phase != 6 || routeSections == 0 || routes == 0 || routesInSection == 0 || lastRouteGlyph != "└" || !sharedChunks || !otherChunks || legends == 0 {
		return input, false, "next_build_incomplete_transcript"
	}

	var summary strings.Builder
	fmt.Fprintf(&summary, "Next.js Build: SUCCESS (%d routes", routes)
	if appRoutes, pagesRoutes := routeCounts["app"], routeCounts["pages"]; appRoutes > 0 || pagesRoutes > 0 {
		fmt.Fprintf(&summary, ": %d app, %d pages", appRoutes, pagesRoutes)
	}
	if sharedSize != "" {
		fmt.Fprintf(&summary, "; shared First Load JS %s", sharedSize)
	}
	if duration != "" {
		fmt.Fprintf(&summary, " in %s", duration)
	}
	summary.WriteString(")\n")
	compacted := []byte(summary.String())
	if len(compacted) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return compacted, true, fmt.Sprintf("compressed Next.js build summary (%d routes)", routes)
}
