// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/harness2export"
)

const labReceiptRel = "benchmark/harness2-lab/receipt.json"

// runHarness2Lab runs the local lab and writes the receipt into the checkout.
// The receipt is the project effect. Push the commit so the remote carries it.
// Routing policy is not modified.
func runHarness2Lab(args []string) int {
	jsonOut := false
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		fmt.Fprintf(os.Stderr, "unknown flag %q\n", a)
		return 2
	}
	root, err := downshiftModuleRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness2 lab: %v\n", err)
		return 1
	}
	rows := harness2MatrixRows()
	receipt, err := harness2export.BuildReceipt(rows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness2 lab judge: %v\n", err)
		return 1
	}
	dataset := filepath.Join(root, filepath.FromSlash(harness2export.SmokeDataset))
	fmt.Fprintf(os.Stderr, "harness2 lab: smoke dataset %s is not a promotion gate\n", harness2export.SmokeDataset)
	var smokeRC int
	discardStdout(func() {
		smokeRC = runBenchmark([]string{dataset, "--report"})
	})
	if smokeRC != 0 {
		fmt.Fprintf(os.Stderr, "harness2 lab: smoke eval failed (exit %d)\n", smokeRC)
		return smokeRC
	}
	body, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness2 lab: %v\n", err)
		return 1
	}
	body = append(body, '\n')
	dest := filepath.Join(root, filepath.FromSlash(labReceiptRel))
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "harness2 lab: %v\n", err)
		return 1
	}
	if err := os.WriteFile(dest, body, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "harness2 lab: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "harness2 lab: wrote %s\n", dest)
	fmt.Fprintf(os.Stderr, "harness2 lab: mutation=%s observe=%s classifier_only=%s\n",
		strings.Join(receipt.MutationTargets, ","),
		strings.Join(receipt.ObservationTargets, ","),
		strings.Join(receipt.ClassifierOnlyTargets, ","))
	if jsonOut {
		os.Stdout.Write(body)
	}
	return 0
}

func harness2MatrixRows() []harness2export.Row {
	hookPresent := func(id string) bool {
		e, ok := harnessRegistry[id]
		return ok && e.Hook != nil
	}
	return harness2export.Matrix(hookPresent, harness2Caps)
}

func discardStdout(fn func()) {
	old := os.Stdout
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		fn()
		return
	}
	os.Stdout = devNull
	fn()
	os.Stdout = old
	devNull.Close()
}

func downshiftModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		mod := filepath.Join(dir, "go.mod")
		body, err := os.ReadFile(mod)
		if err == nil && bytes.Contains(body, []byte("github.com/tiagovilasboas/downshift")) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside the downshift module; run from the repository checkout")
		}
		dir = parent
	}
}
