// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/harness2export"
)

// runHarness2 is the project-facing entry for Harness² maturation (see
// docs/guides/recursive-harnessing.md). It does not call Google Harness²;
// harness-core owns the upstream install.
func runHarness2(args []string) int {
	if len(args) == 0 {
		printHarness2Usage(os.Stderr)
		return 2
	}
	switch args[0] {
	case "matrix":
		return runHarness2Matrix(args[1:])
	case "eval":
		return runHarness2Eval(args[1:])
	case "lab":
		return runHarness2Lab(args[1:])
	case "help", "-h", "--help":
		printHarness2Usage(os.Stdout)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown harness2 subcommand %q\n", args[0])
		printHarness2Usage(os.Stderr)
		return 2
	}
}

func printHarness2Usage(w *os.File) {
	fmt.Fprintln(w, "usage:")
	fmt.Fprintln(w, "  downshift harness2 matrix [--json]   Lab matrix: role, maturity, mutation eligibility")
	fmt.Fprintln(w, "  downshift harness2 eval <dataset.json> [--json]   Classifier KPIs (public file is smoke, not a promotion gate)")
	fmt.Fprintln(w, "  downshift harness2 lab [--json]   Local lab: judge the matrix, smoke-eval, write benchmark/harness2-lab/receipt.json")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Smoke dataset: benchmark/routing-tasks.public.json")
	fmt.Fprintln(w, "Promotion datasets stay in downshift-labs. Push the receipt so the remote checkout matches this lab run.")
}

func runHarness2Matrix(args []string) int {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		fmt.Fprintf(os.Stderr, "unknown flag %q\n", a)
		return 2
	}
	rows := harness2MatrixRows()
	if jsonOut {
		payload := struct {
			SchemaVersion int                  `json:"schema_version"`
			GeneratedAt   string               `json:"generated_at"`
			Product       string               `json:"product"`
			Harnesses     []harness2export.Row `json:"harnesses"`
			Docs          string               `json:"docs"`
		}{
			SchemaVersion: 1,
			GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
			Product:       "downshift",
			Harnesses:     rows,
			Docs:          "docs/guides/recursive-harnessing.md",
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return boolExit(enc.Encode(payload))
	}
	for _, row := range rows {
		elig := "no"
		if row.RecursiveHarness2Eligible {
			elig = "yes"
		}
		hook := "no"
		if row.HasHook {
			hook = "yes"
		}
		fmt.Printf("%-14s role=%-16s maturity=%-14s hook=%s eligible=%s\n", row.ID, row.Role, row.Maturity, hook, elig)
	}
	return 0
}

func runHarness2Eval(args []string) int {
	// Same flags as benchmark --report; stable JSON for harness-core scripts.
	fwd := make([]string, 0, len(args)+1)
	for _, a := range args {
		if a == "--json" {
			fwd = append(fwd, "--report")
			continue
		}
		fwd = append(fwd, a)
	}
	if len(fwd) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift harness2 eval <dataset.json> [--json]")
		return 2
	}
	for _, a := range fwd {
		if filepath.Base(a) == "routing-tasks.public.json" {
			fmt.Fprintf(os.Stderr, "harness2 eval: %s is a smoke fixture, not a promotion gate\n", a)
			break
		}
	}
	return runBenchmark(fwd)
}

func harness2Caps(id string) (harness2export.CapsSnapshot, bool) {
	c, ok := core.CapsFor(id)
	if !ok && id == "grok" {
		c = core.GrokCaps
		ok = true
	}
	if !ok {
		return harness2export.CapsSnapshot{}, false
	}
	return harness2export.CapsSnapshot{
		CanRewriteModel:        c.CanRewriteModel,
		CanApplyEffort:         c.CanApplyEffort,
		SingleModelReasoning:   c.SingleModelReasoning,
		CanRewriteShellCommand: c.CanRewriteShellCommand,
	}, true
}

func boolExit(err error) int {
	if err != nil {
		fmt.Fprintf(os.Stderr, "json encode: %v\n", err)
		return 1
	}
	return 0
}
