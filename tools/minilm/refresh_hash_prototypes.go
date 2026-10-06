// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/downshift/internal/semantic"
)

func main() {
	root, _ := os.Getwd()
	tasks := filepath.Join(root, "benchmark", "tasks.json")
	out := filepath.Join(root, "internal", "semantic", "data", "prototypes.json")
	if err := semantic.RefreshHashPrototypesFromBenchmark(tasks, out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("ok", out)
}
