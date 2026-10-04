// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

// dsmon-server — standalone binary for the harness-hub dashboard.
// Prefer `downshift serve` for the integrated experience.
//
// Build:  go build -o dsmon-server ./cmd/dsmon-server
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tiagovilasboas/harness-downshift/internal/server"
)

func main() {
	// Look for web/ next to the binary
	exe, _ := os.Executable()
	webDir := filepath.Join(filepath.Dir(exe), "web")
	if _, err := os.Stat(webDir); err != nil {
		webDir = "web" // fallback to CWD/web
	}
	if err := server.Run(server.DefaultPort, webDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
