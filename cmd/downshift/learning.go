// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/compressor/learning"
)

// runLearning is `downshift learning [subcommand]`: manage the compressor learning store.
// Subcommands:
//   - suggestions: show commands that would benefit from a compressor
//   - stats: show learning store statistics
//   - export: dump the store as JSON (shapes and counts, no output text)
//   - clear: reset the learning store
func runLearning(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		args = []string{"suggestions"}
	}

	switch args[0] {
	case "suggestions", "suggest":
		return learningShowSuggestions(stdout, stderr)
	case "stats":
		return learningShowStats(stdout, stderr)
	case "export":
		return learningExport(stdout, stderr)
	case "clear":
		return learningClear(stdout, stderr)
	default:
		fmt.Fprintf(stderr, "downshift learning: unknown subcommand %q\n", args[0])
		fmt.Fprintln(stderr, "usage: downshift learning [suggestions|stats|export|clear]")
		return 2
	}
}

func learningShowSuggestions(stdout, stderr io.Writer) int {
	suggestions, err := learning.Suggestions(10)
	if err != nil {
		fmt.Fprintf(stderr, "downshift learning: %v\n", err)
		return 1
	}

	if len(suggestions) == 0 {
		fmt.Fprintln(stdout, "No compressor suggestions yet.")
		fmt.Fprintln(stdout, "Run more commands through `downshift compact` to identify patterns.")
		return 0
	}

	fmt.Fprintln(stdout, "Compressor suggestions (highest impact first):")
	fmt.Fprintln(stdout)

	for i, s := range suggestions {
		cmd := strings.Join(s.Signature.Words, " ")
		avgKB := float64(s.AvgBytes) / 1024
		totalKB := float64(s.TotalBytes) / 1024

		marker := "  "
		if s.Recommended {
			marker = "→ "
		}

		fmt.Fprintf(stdout, "%s%d. %s\n", marker, i+1, cmd)
		fmt.Fprintf(stdout, "      Count: %d runs · Avg: %.1f KB · Total: %.1f KB\n", s.Count, avgKB, totalKB)
		if s.Family != "" {
			fmt.Fprintf(stdout, "      Family: %s\n", s.Family)
		}
		if sh := s.Shape; sh != nil {
			fmt.Fprintf(stdout, "      Shape: %d lines · %d path-like · %d url-like · %d status-like · %d number-heavy\n",
				sh.Lines, sh.PathLike, sh.URLLike, sh.StatusLike, sh.NumberHeavy)
		}
		fmt.Fprintln(stdout)
	}

	if len(suggestions) > 0 && suggestions[0].Recommended {
		fmt.Fprintln(stdout, "→ = recommended for new compressor (high count + high bytes)")
	}

	return 0
}

func learningShowStats(stdout, stderr io.Writer) int {
	stats, err := learning.Stats()
	if err != nil {
		fmt.Fprintf(stderr, "downshift learning: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "Compressor Learning Stats")
	fmt.Fprintln(stdout, "─────────────────────────")
	fmt.Fprintf(stdout, "Unique signatures:  %d\n", stats.TotalSignatures)
	fmt.Fprintf(stdout, "Total commands:     %d\n", stats.TotalCommands)
	fmt.Fprintf(stdout, "Total bytes:        %.1f KB\n", float64(stats.TotalBytes)/1024)
	fmt.Fprintf(stdout, "Ready to suggest:   %d\n", stats.SuggestableCount)
	if stats.UpdatedAt != "" {
		fmt.Fprintf(stdout, "Last updated:       %s\n", stats.UpdatedAt)
	}

	return 0
}

func learningExport(stdout, stderr io.Writer) int {
	store, err := learning.Export()
	if err != nil {
		fmt.Fprintf(stderr, "downshift learning: %v\n", err)
		return 1
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "downshift learning: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, string(data))
	return 0
}

func learningClear(stdout, stderr io.Writer) int {
	if err := learning.Clear(); err != nil {
		fmt.Fprintf(stderr, "downshift learning: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "Learning store cleared.")
	return 0
}
