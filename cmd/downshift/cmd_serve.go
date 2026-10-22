// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tiagovilasboas/downshift/internal/api"
)

// runServe is `downshift serve`: start the API server for the dashboard.
// The server exposes /api/session/current and /api/session/{id} endpoints.
func runServe(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", 8787, "port to listen on (0 for auto)")
	quiet := fs.Bool("q", false, "suppress startup message")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: downshift serve [--port=PORT] [-q]")
		return 2
	}

	srv, err := api.NewServer(*port)
	if err != nil {
		fmt.Fprintf(stderr, "downshift serve: %v\n", err)
		return 1
	}

	// Handle shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx)
	}()

	if !*quiet {
		fmt.Fprintf(stdout, "Downshift API server listening on http://%s\n", srv.Addr())
		fmt.Fprintf(stdout, "  GET /api/session/current  - current session panel\n")
		fmt.Fprintf(stdout, "  GET /api/session/{id}     - specific session panel\n")
		fmt.Fprintf(stdout, "  GET /api/health           - health check\n")
		fmt.Fprintf(stdout, "\nPress Ctrl+C to stop\n")
	}

	if err := srv.Start(); err != nil && ctx.Err() == nil {
		fmt.Fprintf(stderr, "downshift serve: %v\n", err)
		return 1
	}

	return 0
}
