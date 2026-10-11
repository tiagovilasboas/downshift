// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/adapt"
	"github.com/tiagovilasboas/downshift/internal/outcome"
)

func runEvalAdapt(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("eval-adapt", flag.ContinueOnError)
	fs.SetOutput(stderr)
	tasksDir := fs.String("tasks", "internal/outcome/testdata", "task directory")
	runsDir := fs.String("runs", "", "recorded runs directory (required)")
	priceSmall := fs.String("price-small", "", "small tier USD per 1M tokens as in,out (enables cost report)")
	priceMid := fs.String("price-mid", "", "mid tier USD per 1M tokens as in,out")
	priceFrontier := fs.String("price-frontier", "", "frontier tier USD per 1M tokens as in,out")
	refIn := fs.Int("ref-in", 20000, "reference request input tokens per attempt")
	refOut := fs.Int("ref-out", 1000, "reference request output tokens per attempt")
	failPenalty := fs.Float64("fail-penalty", 0, "USD cost of one failed attempt beyond tokens (0 = escalation is free)")
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
	c, err := adapt.EvalSharedShapeMemoryLOO(tasks, runs)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "eval-adapt tasks=%s runs=%s\n", filepath.Clean(*tasksDir), filepath.Clean(*runsDir))
	printReport(stdout, "A_contrafactual_per_task", a)
	printReport(stdout, "B_shared_shape_memory", b)
	printReport(stdout, "C_shared_shape_memory_loo", c)
	if *priceSmall == "" && *priceMid == "" && *priceFrontier == "" {
		return 0
	}
	cm, err := costModelFromFlags(*priceSmall, *priceMid, *priceFrontier, *refIn, *refOut)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	cm.FailurePenalty = *failPenalty
	if err := cm.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	rows, err := adapt.EvalCost(tasks, runs, cm)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintf(stdout, "cost ref_in=%d ref_out=%d fail_penalty=%.4f (USD, escalate one tier up on failure)\n", *refIn, *refOut, *failPenalty)
	for _, r := range rows {
		fmt.Fprintf(stdout, "%-16s total=%d first_pass=%d completed=%d attempts=%d cost=%.4f per_completed=%.4f\n",
			r.Name, r.Total, r.FirstPass, r.Completed, r.Attempts, r.Cost, r.PerCompleted())
	}
	return 0
}

func costModelFromFlags(small, mid, frontier string, refIn, refOut int) (adapt.CostModel, error) {
	cm := adapt.CostModel{RefInputTok: refIn, RefOutputTok: refOut}
	for i, raw := range []string{small, mid, frontier} {
		p, err := parsePrice(raw)
		if err != nil {
			return cm, fmt.Errorf("price for tier %d: %w", i, err)
		}
		cm.Prices[i] = p
	}
	return cm, cm.Validate()
}

func parsePrice(raw string) (adapt.TierPrice, error) {
	parts := strings.Split(raw, ",")
	if len(parts) != 2 {
		return adapt.TierPrice{}, fmt.Errorf("want in,out got %q", raw)
	}
	in, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	out, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return adapt.TierPrice{}, fmt.Errorf("want numeric in,out got %q", raw)
	}
	return adapt.TierPrice{InputM: in, OutputM: out}, nil
}

func printReport(w io.Writer, label string, r adapt.OutcomeEvalReport) {
	fmt.Fprintf(w, "%s total=%d baseline_pass=%d (%.1f%%) adjusted_pass=%d (%.1f%%) down=%d up=%d same=%d\n",
		label, r.Total, r.BaselinePass, r.BaselineRate()*100, r.AdjustedPass, r.AdjustedRate()*100,
		r.Down, r.Up, r.Same)
}
