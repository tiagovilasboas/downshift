// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package postrun

import (
	"fmt"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/hookport"
)

// SearchBudget bounds the text a cut search result keeps, before its marker:
// a result over execwrap.BalanceThreshold reaches the model under it.
const SearchBudget = 12 << 10

// compactSearch cuts a search tool's matches and its file list, each only
// when larger than the balanced threshold and only once its raw text is
// stored. Matches keep the first of each file and a count of the rest
// (compressor.BalanceMatches); a file list keeps its head. Each part is then
// held to SearchBudget, and says how much it left out and the retrieve id.
func compactSearch(in hookport.ToolOutput, o Options) Result {
	raw := len(in.Content)
	for _, p := range in.Paths {
		raw += len(p) + 1
	}
	if raw == 0 {
		return Result{}
	}
	res := Result{Considered: true, Family: FamilySearchTool, Output: in, RawBytes: int64(raw), Passthrough: execwrap.PassthroughSmallOutput}
	emitted := raw
	if !o.Balanced || o.Keep == nil {
		return settle(res, in)
	}
	if in.HasContent && len(in.Content) > execwrap.BalanceThreshold {
		if cut, id, ok := cutMatches(in.Content, o.Keep); ok {
			res.Output.Content, res.KeptID = cut, id
			emitted += len(cut) - len(in.Content)
		}
	}
	if in.HasPaths && len(strings.Join(in.Paths, "\n")) > execwrap.BalanceThreshold {
		if cut, id, ok := cutPaths(in.Paths, o.Keep); ok {
			before := len(strings.Join(in.Paths, "\n")) + 1
			res.Output.Paths = cut
			emitted += len(strings.Join(cut, "\n")) + 1 - before
			if res.KeptID == "" {
				res.KeptID = id
			}
		}
	}
	if res.KeptID != "" {
		res.Compacted, res.Folded, res.Passthrough = true, true, ""
	}
	res.EmittedBytes = int64(emitted)
	return settle(res, in)
}

// cutMatches cuts match lines and stores the raw text first.
func cutMatches(text string, keep func([]byte) (string, bool)) (string, string, bool) {
	cut, applied := compressor.Balance(compressor.BalanceMatches, []byte(text))
	if !applied {
		return "", "", false
	}
	lines := strings.Split(strings.TrimSuffix(string(cut), "\n"), "\n")
	// The cut ends with its count of the rest per file ("… N more matches in
	// M files" and one line per file); that summary is kept whole and the
	// matches above it are held to what is left of the budget.
	i := len(lines)
	for j, l := range lines {
		if strings.HasPrefix(l, "… ") {
			i = j
			break
		}
	}
	summary := lines[i:]
	body := append(fitLines(lines[:i], "matches", SearchBudget-len(strings.Join(summary, "\n"))), summary...)
	id, ok := keep([]byte(text))
	if !ok {
		return "", "", false
	}
	body = append(body, strings.TrimSuffix(execwrap.MarkerWithID(len(text), len(strings.Join(body, "\n")), id), "\n"))
	return strings.Join(body, "\n") + "\n", id, true
}

// cutPaths keeps the head of a file list and stores the whole list first.
func cutPaths(paths []string, keep func([]byte) (string, bool)) ([]string, string, bool) {
	body := fitLines(paths, "files", SearchBudget)
	if len(body) >= len(paths) {
		return nil, "", false
	}
	full := strings.Join(paths, "\n") + "\n"
	id, ok := keep([]byte(full))
	if !ok {
		return nil, "", false
	}
	return append(body, strings.TrimSuffix(execwrap.MarkerWithID(len(full), len(strings.Join(body, "\n")), id), "\n")), id, true
}

// fitLines keeps whole lines while they fit in budget bytes and ends with one
// line that counts the rest (what names the unit: "matches", "files").
func fitLines(lines []string, what string, budget int) []string {
	size := 0
	for i, l := range lines {
		size += len(l) + 1
		if size > budget {
			out := append([]string{}, lines[:i]...)
			return append(out, fmt.Sprintf("… %d more %s (downshift retrieve)", len(lines)-i, what))
		}
	}
	return lines
}
