// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
)

// RuboCop default ("clang") formatter output:
//
//	Inspecting 2 files
//	.C
//
//	Offenses:
//
//	app/a.rb:1:1: C: [Correctable] Style/Foo: message
//	puts 'x'
//	^
//
//	2 files inspected, 1 offense detected, 1 offense autocorrectable
var (
	reRubocopInspecting = lazyre.New(`^Inspecting \d+ files?$`)
	reRubocopProgress   = lazyre.New(`^[.CWEFA]+$`)
	reRubocopCaret      = lazyre.New(`^\s*\^~*$`)
	// file:line:col: S: [Correctable] Department/Cop: message
	reRubocopOffense = lazyre.New(`^(\S+?):(\d+):(\d+): ([CWEF]): (?:\[\w+\] )?([A-Z][A-Za-z]*(?:/[A-Za-z0-9]+)+): .*$`)
	reRubocopSummary = lazyre.New(`^(\d+) files? inspected, (?:no offenses|(\d+) offenses?) detected(?:, \d+ offenses? autocorrectable)?$`)

	// Whole-input fingerprints for detection.
	reRubocopSummaryAny    = lazyre.New(`(?m)^\d+ files? inspected, (?:no offenses|\d+ offenses?) detected`)
	reRubocopInspectingAny = lazyre.New(`(?m)^Inspecting \d+ files?$`)
)

func isRubocop(input []byte) bool {
	return reRubocopSummaryAny.Match(input) && reRubocopInspectingAny.Match(input)
}

// rubocopOffenses is the parse of a transcript: per-file and per-cop counts in
// first-seen order, and every error/fatal offence block verbatim.
type rubocopOffenses struct {
	fileOrder []string
	perFile   map[string]map[string]int
	perCop    map[string]int
	severe    []string
	total     int
}

func (o *rubocopOffenses) add(m []string) {
	if o.perFile[m[1]] == nil {
		o.perFile[m[1]] = map[string]int{}
		o.fileOrder = append(o.fileOrder, m[1])
	}
	o.perFile[m[1]][m[5]]++
	o.perCop[m[5]]++
	o.total++
}

// parseRubocop reads a transcript of the default formatter. It reports false on
// any line that is not part of it, so the caller keeps the raw output.
func parseRubocop(lines []string) (*rubocopOffenses, string, bool) {
	o := &rubocopOffenses{perFile: map[string]map[string]int{}, perCop: map[string]int{}}
	inBlock, severe, ctx := false, false, 0
	summary := ""
	for _, line := range lines {
		switch {
		case summary != "":
			return nil, "", false
		case strings.TrimSpace(line) == "":
			continue
		case reRubocopSummary.MatchString(line):
			summary = line
		case line == "Offenses:":
			inBlock = true
		case inBlock && reRubocopOffense.MatchString(line):
			m := reRubocopOffense.FindStringSubmatch(line)
			o.add(m)
			severe, ctx = m[4] == "E" || m[4] == "F", 0
			if severe {
				o.severe = append(o.severe, line)
			}
		case inBlock && ctx == 0, inBlock && ctx == 1 && reRubocopCaret.MatchString(line):
			// The source line, then the caret line, of the offence above. Any
			// other extra line (a multi-line range, an injected message) is
			// not part of the shape this compressor knows.
			ctx++
			if severe {
				o.severe = append(o.severe, line)
			}
		case reRubocopInspecting.MatchString(line), reRubocopProgress.MatchString(line):
		default:
			return nil, "", false
		}
	}
	return o, summary, summary != ""
}

// compressRubocop groups offences by file and by cop with counts, keeps every
// error and fatal offence (header, source line, caret) verbatim and the summary
// line as written. A clean run keeps only the summary line.
func compressRubocop(input []byte) ([]byte, bool, string) {
	if len(input) == 0 || len(input) > MaxProcessableBytes || input[len(input)-1] != '\n' || !utf8.Valid(input) || strings.ContainsAny(string(input), "\r\x00\x1b") {
		return input, false, "rubocop_invalid_or_incomplete_input"
	}
	o, summary, ok := parseRubocop(strings.Split(strings.TrimSuffix(string(input), "\n"), "\n"))
	if !ok {
		return input, false, "rubocop_unrecognized_line"
	}
	declared := reRubocopSummary.FindStringSubmatch(summary)[2]
	if fmt.Sprint(o.total) != declared && !(declared == "" && o.total == 0) {
		return input, false, "rubocop_count_mismatch"
	}
	out := o.render(summary)
	if len(out) >= len(input) {
		return input, false, "no_compression_gain"
	}
	return out, true, fmt.Sprintf("compressed rubocop (%d files)", len(o.fileOrder))
}

func (o *rubocopOffenses) render(summary string) []byte {
	var b strings.Builder
	for _, f := range o.fileOrder {
		fmt.Fprintf(&b, "%s: %s\n", f, countList(o.perFile[f]))
	}
	if len(o.perCop) > 0 {
		fmt.Fprintf(&b, "cops: %s\n", countList(o.perCop))
	}
	for _, l := range o.severe {
		b.WriteString(l + "\n")
	}
	b.WriteString(summary + "\n")
	return []byte(b.String())
}

// countList renders "Cop n, Cop n" by count descending, then name.
func countList(counts map[string]int) string {
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("%s %d", n, counts[n])
	}
	return strings.Join(parts, ", ")
}
