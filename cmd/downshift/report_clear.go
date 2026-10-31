// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

// reportClearFiles are the measurement logs `downshift report clear` moves
// out: routing and child-end events, compaction and context records, and the
// compressor learning store. Configuration and caches that change behaviour
// (config, policy, session models, model discovery, quota, scores) stay.
var reportClearFiles = []string{
	"events.jsonl",
	"agents.jsonl",
	"exec-events.jsonl",
	"context-compactions.jsonl",
	"context-sensor.json",
	"loop-events.jsonl",
	"dci-events.jsonl",
	"orchestration-events.jsonl",
	"probe.jsonl",
	"live.json",
	"stats-export.json",
	"compressor-learning.json",
}

const reportClearUsage = "usage: downshift report clear [--dry-run] [--yes]\n"

// runReportClear is `downshift report clear`. It never deletes: the files move
// to backups/report-clear-<time> inside the state directory, so a clear can be
// undone by moving them back. Without --yes it asks, and it refuses when
// standard input is not a terminal.
func runReportClear(args []string, in io.Reader, stdout, stderr io.Writer, now time.Time) int {
	dryRun, yes := false, false
	for _, a := range args {
		switch a {
		case "--dry-run":
			dryRun = true
		case "--yes", "-y":
			yes = true
		default:
			fmt.Fprintf(stderr, "unknown flag: %s\n%s", a, reportClearUsage)
			return 2
		}
	}
	dir, err := paths.StateDir()
	if err != nil {
		fmt.Fprintln(stderr, "report clear:", err)
		return 1
	}
	found, total := reportClearFound(dir)
	if len(found) == 0 {
		fmt.Fprintf(stdout, "Nothing to clear in %s.\n", paths.DisplayStateDir())
		return 0
	}
	backup := filepath.Join(dir, "backups", "report-clear-"+now.UTC().Format("20060102T150405Z"))
	fmt.Fprintf(stdout, "These measurement files in %s will move to %s:\n", paths.DisplayStateDir(), backup)
	for _, f := range found {
		fmt.Fprintf(stdout, "  %-28s %s\n", f.name, humanBytes(f.size))
	}
	fmt.Fprintf(stdout, "  total %s. Config, policy, session models, model discovery and quota are kept.\n", humanBytes(total))
	if dryRun {
		fmt.Fprintln(stdout, "Dry run: nothing was moved.")
		return 0
	}
	if !yes {
		if !isInteractiveTerminal(in) {
			fmt.Fprintln(stderr, "report clear: standard input is not a terminal; pass --yes to clear without asking")
			return 2
		}
		fmt.Fprint(stdout, "Clear them? [y/N] ")
		line, _ := bufio.NewReader(in).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(line)); a != "y" && a != "yes" {
			fmt.Fprintln(stdout, "Nothing was moved.")
			return 0
		}
	}
	if err := os.MkdirAll(backup, 0o700); err != nil {
		fmt.Fprintln(stderr, "report clear:", err)
		return 1
	}
	failed := 0
	for _, f := range found {
		if err := os.Rename(filepath.Join(dir, f.name), filepath.Join(backup, f.name)); err != nil {
			fmt.Fprintf(stderr, "report clear: %s: %v\n", f.name, err)
			failed++
		}
	}
	moved := len(found) - failed
	fmt.Fprintf(stdout, "Moved %d of %d files. The routed vs control comparison and the compaction totals start again from zero.\n", moved, len(found))
	fmt.Fprintf(stdout, "To undo, move the files back from %s.\n", backup)
	if failed > 0 {
		return 1
	}
	return 0
}

type clearFile struct {
	name string
	size int64
}

// reportClearFound lists the measurement files that exist, regular files only.
func reportClearFound(dir string) ([]clearFile, int64) {
	var out []clearFile
	var total int64
	for _, name := range reportClearFiles {
		st, err := os.Lstat(filepath.Join(dir, name))
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		out = append(out, clearFile{name: name, size: st.Size()})
		total += st.Size()
	}
	return out, total
}

// humanBytes renders a byte count for people: B, KB or MB.
func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
