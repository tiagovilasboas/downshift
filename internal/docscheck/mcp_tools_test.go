// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package docscheck_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
)

var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7,
	"eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18,
	"nineteen": 19, "twenty": 20,
}

var toolCount = regexp.MustCompile(`(?i)\b(\d+|[a-z]+)\s+(?:MCP\s+)?tools\b`)

// registeredToolNames is the registry the server serves: the same source as
// TestShippedTextNamesOnlyRegisteredTools.
func registeredToolNames(t *testing.T) []string {
	t.Helper()
	var names []string
	for _, tl := range append(tools.Info(tools.Deps{}), tools.WorkTools(tools.Deps{})...) {
		names = append(names, tl.Name)
	}
	if len(names) == 0 {
		t.Fatal("the MCP server registers no tools")
	}
	return names
}

// toolCountProblems flags a count of tools, in digits or as a word, on a line
// that talks about MCP when it differs from want.
func toolCountProblems(lines []docLine, want int) []string {
	var bad []string
	for _, l := range lines {
		// CHANGELOG.md records the count each release had.
		if l.File == "CHANGELOG.md" || !strings.Contains(strings.ToLower(l.Text), "mcp") {
			continue
		}
		for _, m := range toolCount.FindAllStringSubmatch(l.Text, -1) {
			n, err := strconv.Atoi(m[1])
			if err != nil {
				var ok bool
				if n, ok = numberWords[strings.ToLower(m[1])]; !ok {
					continue // "the tools", "worker tools"
				}
			}
			if n != want {
				bad = append(bad, l.File+":"+strconv.Itoa(l.N)+" says "+m[0]+", the server registers "+strconv.Itoa(want))
			}
		}
	}
	return bad
}

// missingFromSection returns the tools whose backticked name is absent from the
// `downshift mcp serve` section of cli.md.
func missingFromSection(cli string, names []string) []string {
	_, section, ok := strings.Cut(cli, "### `downshift mcp serve`")
	if !ok {
		return []string{"section ### `downshift mcp serve`"}
	}
	if end := strings.Index(section, "\n### "); end >= 0 {
		section = section[:end]
	}
	var missing []string
	for _, n := range names {
		if !strings.Contains(section, "`"+n+"`") && !strings.Contains(section, "`"+n+" {") {
			missing = append(missing, n)
		}
	}
	return missing
}

// The tool list in cli.md and every count of MCP tools in the docs match the
// registry.
func TestDocumentedMCPToolsMatchTheRegistry(t *testing.T) {
	root := repoRoot(t)
	names := registeredToolNames(t)
	for _, n := range missingFromSection(mustRead(t, root+"/docs/reference/cli.md"), names) {
		t.Errorf("docs/reference/cli.md does not list the registered MCP tool %s", n)
	}
	for _, b := range toolCountProblems(guardedLines(t, root), len(names)) {
		t.Errorf("%s; fix the doc or mark the line %s", b, historyMarker)
	}
}

func TestMCPToolGuardBites(t *testing.T) {
	lines := []docLine{{"README.md", 218, "MCP server: eleven tools."}, {"x.md", 1, "MCP: 13 tools"}}
	if got := toolCountProblems(lines, 13); len(got) != 1 {
		t.Errorf("got %v, want only eleven flagged", got)
	}
	if got := missingFromSection("### `downshift mcp serve`\n`providers {a}` `plan`\n", []string{"providers", "plan", "retrieve"}); len(got) != 1 {
		t.Errorf("got %v, want retrieve missing", got)
	}
}
