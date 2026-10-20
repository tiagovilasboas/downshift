// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// dsmon-server — standalone binary for the harness-hub dashboard.
// Prefer `downshift serve` for the integrated experience.
//
// Build:  go build -o dsmon-server ./cmd/dsmon-server
package main

import (
	"fmt"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"os"

	"github.com/tiagovilasboas/downshift/internal/server"
)

func main() {
	telemetry.DirectionCatalog = catalog.Load()
	if err := server.Run(server.DefaultPort, server.WebRoot()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
