// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func runDoctor(w io.Writer, cat *catalog.Catalog) int {
	dir, src, err := paths.Resolve()
	if err != nil {
		fmt.Fprintf(w, "state_dir: error: %v\n", err)
		return 1
	}
	fmt.Fprintf(w, "version: downshift %s\n", buildVersion)
	if buildCommit != "" {
		fmt.Fprintf(w, "commit: %s\n", buildCommit)
	}
	fmt.Fprintf(w, "state_dir: %s\n", dir)
	fmt.Fprintf(w, "state_dir_source: %s\n", src)
	if src == paths.SourceLegacy {
		fmt.Fprintf(w, "migration_hint: copy %s to %s/%s when ready (or set %s)\n",
			dir, os.Getenv("HOME"), paths.DirName, paths.EnvStateDir)
	}

	catalogPath, _ := paths.CatalogPath()
	if _, err := os.Stat(catalogPath); err == nil {
		fmt.Fprintf(w, "catalog: user override at %s\n", catalogPath)
	} else {
		fmt.Fprintf(w, "catalog: embedded default (override path %s)\n", catalogPath)
	}

	eventsPath, _ := paths.EventsPath()
	if _, err := os.Stat(eventsPath); err == nil {
		fmt.Fprintf(w, "events: %s (present)\n", eventsPath)
	} else {
		fmt.Fprintf(w, "events: %s (not created yet)\n", eventsPath)
	}

	_ = cat // reserved for future: effective harness count, shadow flag, etc.
	return 0
}
