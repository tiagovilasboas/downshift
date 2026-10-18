// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/tiagovilasboas/downshift/internal/models"
)

// runModels dispatches the 'models' subcommands: list, check, pull.
func runModels(catalog models.CatalogReader, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift models <list|check|pull|discover|scores>")
		return 2
	}
	switch args[0] {
	case "list":
		models.List(catalog, os.Stdout)
		return 0
	case "check":
		return models.Check(catalog, os.Stdout, os.Stderr)
	case "pull":
		return models.Pull(catalog, os.Stdout, os.Stderr)
	case "discover":
		return runModelsDiscover(catalog, args[1:], os.Stdout, os.Stderr)
	case "scores":
		return runModelsScores(args[1:], os.Stdout, os.Stderr)
	default:
		fmt.Fprintf(os.Stderr, "unknown models subcommand %q\n", args[0])
		return 2
	}
}
