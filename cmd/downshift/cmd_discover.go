// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/tiagovilasboas/downshift-ai/internal/discover"
)

// runDiscover scans harness transcripts for commands the compactor would have
// changed but that ran bare. It only parses flags and renders; the scan lives
// in internal/discover and never writes what it reads. The home directory is
// resolved here and passed in.
func runDiscover(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("discover", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	since := fs.String("since", "7d", "")
	harness := fs.String("harness", discover.HarnessAll, "")
	top := fs.Int("top", 10, "")
	asJSON := fs.Bool("json", false, "")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || *top < 0 {
		fmt.Fprintln(stderr, "usage: downshift discover [--since 7d] [--harness claude-code|codex|all] [--top N] [--json]")
		return 2
	}
	window, err := discover.ParseSince(*since)
	if err != nil {
		fmt.Fprintf(stderr, "discover: %v\n", err)
		return 2
	}
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(stderr, "discover: cannot resolve the home directory: %v\n", err)
		return 1
	}
	rep, err := discover.Scan(discover.Options{Home: home, Harness: *harness, Since: window, Label: *since, Top: *top})
	if err != nil {
		fmt.Fprintf(stderr, "discover: %v\n", err)
		return 2
	}
	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return 1
		}
		return 0
	}
	renderDiscover(stdout, rep)
	return 0
}

// renderDiscover prints the report as plain text.
func renderDiscover(w io.Writer, r discover.Report) {
	fmt.Fprintf(w, "Downshift discover: last %s, %d transcript files\n\n", r.Since, r.Files)
	for _, h := range r.Harnesses {
		fmt.Fprintf(w, "%-12s %d commands: %d compacted, %d compactible but bare (%s), %d skipped\n",
			h.Harness, h.Commands, h.Wrapped, h.Bare, humanBytes(h.BareBytes), h.Skipped)
	}
	t := r.Total
	fmt.Fprintf(w, "%-12s %d commands: %d compacted, %d compactible but bare (%s), %d skipped\n",
		"all", t.Commands, t.Wrapped, t.Bare, humanBytes(t.BareBytes), t.Skipped)
	renderCounts(w, "Skipped by reason:", t.SkipReason)
	renderCounts(w, "Metachar skips by kind:", t.MetacharKind)
	if len(r.Top) == 0 {
		fmt.Fprintln(w, "\nNo bare compactible commands found.")
		return
	}
	fmt.Fprintln(w, "\nTop missed (family, command, count, result size):")
	for i, m := range r.Top {
		fmt.Fprintf(w, "  %2d. %-22s %-12s %5d  %s\n", i+1, m.Family, m.Binary, m.Count, humanBytes(m.Bytes))
	}
	fmt.Fprintln(w, "\nRun `downshift enable` or `downshift setup` so these go through the compactor.")
}

// renderCounts prints a titled count table, largest first, names breaking ties.
func renderCounts(w io.Writer, title string, counts map[string]int) {
	if len(counts) == 0 {
		return
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		return counts[a] > counts[b] || counts[a] == counts[b] && a < b
	})
	fmt.Fprintf(w, "\n%s\n", title)
	for _, k := range keys {
		fmt.Fprintf(w, "  %-18s %d\n", k, counts[k])
	}
}
