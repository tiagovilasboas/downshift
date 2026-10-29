// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// knownHarnessNames lists the harnesses Downshift has an adapter for, in a
// stable order. A harness with no events in the window is listed as such.
func knownHarnessNames() []string {
	var names []string
	for _, n := range core.AllHarnessCaps() {
		names = append(names, n.Harness)
	}
	sort.Strings(names)
	return names
}

// harnessesWithoutEvents returns the known harnesses that have no entry in have.
func harnessesWithoutEvents(have map[string]bool) []string {
	var out []string
	for _, n := range knownHarnessNames() {
		if !have[n] {
			out = append(out, n)
		}
	}
	return out
}

// printRoutingWhy writes the "Why spawns changed" section: per harness, how
// each verdict decided and the top reasons. Enums, tiers and declared ids only.
func printRoutingWhy(w io.Writer, why accounting.RoutingWhy) {
	fmt.Fprintf(w, "\nWHY SPAWNS CHANGED (%s)\n", why.Scope.Label())
	if why.Decisions == 0 {
		fmt.Fprintln(w, "   No routing events in this window.")
		return
	}
	have := map[string]bool{}
	for _, h := range why.Harnesses {
		have[h.Harness] = true
		fmt.Fprintf(w, "   %s: %d spawns\n", h.Harness, h.Decisions)
		if h.Recorded > 0 {
			fmt.Fprintf(w, "      by verdict: %s\n", joinCounts(h.ByVerdict))
			fmt.Fprintf(w, "      top reasons: %s\n", joinCounts(h.TopReasons))
		}
		if h.NotRecorded > 0 {
			fmt.Fprintf(w, "      %d without a routing note (written before it was recorded)\n", h.NotRecorded)
		}
	}
	if rest := harnessesWithoutEvents(have); len(rest) > 0 {
		fmt.Fprintf(w, "   no events: %s\n", strings.Join(rest, ", "))
	}
}

// joinCounts renders "kept 3 · rule 2".
func joinCounts(cs []accounting.WhyCount) string {
	parts := make([]string, 0, len(cs))
	for _, c := range cs {
		parts = append(parts, fmt.Sprintf("%s %d", c.Key, c.Count))
	}
	return strings.Join(parts, " · ")
}
