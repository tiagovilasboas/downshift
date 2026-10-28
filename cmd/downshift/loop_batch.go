// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/loop"
)

// runLoopBatch runs the tasks of a batch file in order and prints one summary.
// Exit status: 0 when every task passed, 1 when any failed or was skipped,
// 2 when the file cannot be read.
func runLoopBatch(base loop.Config, path string, continueOnError, asJSON bool, out, errw io.Writer) int {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(errw, "loop: %v\n", err)
		return 2
	}
	defer f.Close()
	items, err := loop.ParseBatch(f)
	if err != nil {
		fmt.Fprintf(errw, "loop: %v\n", err)
		return 2
	}
	res := loop.RunBatch(context.Background(), loop.BatchConfig{
		Items:           items,
		Base:            base,
		ContinueOnError: continueOnError,
	})
	if asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		printBatchSummary(out, res)
	}
	if res.OK() {
		return 0
	}
	return 1
}

func printBatchSummary(out io.Writer, r loop.BatchResult) {
	for _, it := range r.Items {
		mark, note := "✓", ""
		switch {
		case it.Skipped:
			mark, note = "-", "skipped"
		case !it.Success:
			mark, note = "✗", it.Message
		case it.CascadeActive:
			note = "recovered by cascade"
		}
		fmt.Fprintf(out, "%s %d. %s", mark, it.Index, it.Task)
		if note != "" {
			fmt.Fprintf(out, " (%s)", note)
		}
		fmt.Fprintln(out)
	}
	fmt.Fprintf(out, "\n%d passed, %d failed, %d skipped, %d recovered by cascade, %v\n",
		r.Passed, r.Failed, r.Skipped, r.Recovered, r.Duration.Round(time.Millisecond))
}
