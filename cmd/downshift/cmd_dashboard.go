// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/tiagovilasboas/downshift/internal/api"
)

// DefaultDashboardPort is the dedicated port for the Downshift dashboard.
// 1337 = LEET 🤓
const DefaultDashboardPort = 1337

// runDashboard is `downshift dashboard`: start the API server with the SPA.
func runDashboard(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(stderr)
	port := fs.Int("port", DefaultDashboardPort, "port to listen on")
	openBrowser := fs.Bool("open", true, "open browser automatically")
	quiet := fs.Bool("q", false, "suppress startup message")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: downshift dashboard [--port=PORT] [--open=false] [-q]")
		return 2
	}

	srv, err := api.NewDashboardServer(*port)
	if err != nil {
		fmt.Fprintf(stderr, "downshift dashboard: %v\n", err)
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

	url := fmt.Sprintf("http://localhost:%d", srv.Port())
	if !*quiet {
		fmt.Fprintf(stdout, "\n")
		fmt.Fprintf(stdout, "  ┌─────────────────────────────────────────┐\n")
		fmt.Fprintf(stdout, "  │      Downshift Dashboard                │\n")
		fmt.Fprintf(stdout, "  │      %s                     │\n", url)
		fmt.Fprintf(stdout, "  └─────────────────────────────────────────┘\n")
		fmt.Fprintf(stdout, "\n")
		fmt.Fprintf(stdout, "  Press Ctrl+C to stop\n\n")
	}

	if *openBrowser {
		openURL(url)
	}

	if err := srv.Start(); err != nil && ctx.Err() == nil {
		fmt.Fprintf(stderr, "downshift dashboard: %v\n", err)
		return 1
	}

	return 0
}

// openURL opens the URL in the default browser (best effort, no error on failure).
func openURL(url string) {
	// Platform-specific open commands
	var cmd string
	var args []string

	switch {
	case fileExistsForBrowser("/usr/bin/open"): // macOS
		cmd = "open"
		args = []string{url}
	case fileExistsForBrowser("/usr/bin/xdg-open"): // Linux
		cmd = "xdg-open"
		args = []string{url}
	case fileExistsForBrowser("/mnt/c/Windows"): // WSL
		cmd = "cmd.exe"
		args = []string{"/c", "start", url}
	default:
		return // Windows native handled by start command
	}

	// Best effort, ignore errors
	go func() {
		_ = runBrowserCommand(cmd, args...)
	}()
}

func fileExistsForBrowser(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func runBrowserCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}
