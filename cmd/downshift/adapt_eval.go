// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/adapt"
	"github.com/tiagovilasboas/downshift/internal/outcome"
)

func runEvalAdapt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval-adapt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tasksDir := fs.String("tasks", "internal/outcome/testdata", "task directory")
	runsDir := fs.String("runs", "", "recorded runs directory (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *runsDir == "" {
		fmt.Fprintln(stderr, "usage: downshift eval-adapt --runs=<dir> [--tasks=<dir>]")
		return 2
	}
	tasks, err := outcome.Load(*tasksDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	runs, err := outcome.LoadRuns(*runsDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	a, err := adapt.EvalContrafactualPerTask(tasks, runs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	b, err := adapt.EvalSharedShapeMemory(tasks, runs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "eval-adapt tasks=%s runs=%s\n", filepath.Clean(*tasksDir), filepath.Clean(*runsDir))
	printReport(stdout, "A_contrafactual_per_task", a)
	printReport(stdout, "B_shared_shape_memory", b)
	return 0
}

func printReport(w io.Writer, label string, r adapt.OutcomeEvalReport) {
	fmt.Fprintf(w, "%s total=%d baseline_pass=%d (%.1f%%) adjusted_pass=%d (%.1f%%) down=%d up=%d same=%d\n",
		label, r.Total, r.BaselinePass, r.BaselineRate()*100, r.AdjustedPass, r.AdjustedRate()*100,
		r.Down, r.Up, r.Same)
}
