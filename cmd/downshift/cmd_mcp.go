// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/mcp/tools"
	"github.com/tiagovilasboas/downshift/internal/mcp/transport"
	"github.com/tiagovilasboas/downshift/internal/models"
)

const mcpUsage = "usage: downshift mcp serve\n"

// mcpInstructions is what a client shows its model about the server.
const mcpInstructions = "Downshift applies the model you choose for a subagent, within the policy the user declared, and measures the result; " +
	"it never picks a model for you. Call providers to see which models this harness offers right now, playbook for the working rules a worker should receive, " +
	"apply to see what would be applied for a choice, and status for the quota advice. Every result carries a strain block with the quota headroom."

// runMcp is `downshift mcp`. Only `serve` exists: an MCP server on standard
// input and output, started by the harness that registered it.
func runMcp(args []string, cat *catalog.Catalog, stdout, stderr io.Writer) int {
	if len(args) != 1 || args[0] != "serve" {
		fmt.Fprint(stderr, mcpUsage)
		return 2
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "mcp serve:", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return serveMcp(ctx, cat, root, os.Stdin, stdout, stderr)
}

// serveMcp answers one client until it closes its input or ctx ends. Standard
// output carries protocol messages only; every log line goes to errW.
func serveMcp(ctx context.Context, cat *catalog.Catalog, root string, in io.Reader, out, errW io.Writer) int {
	md, ok := any(cat).(models.Metadata)
	if !ok {
		fmt.Fprintln(errW, "mcp serve: the catalog does not expose model metadata")
		return 1
	}
	root = resolvedDir(root)
	broad := broadRoot(root)
	log := slog.New(slog.NewTextHandler(errW, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if broad {
		log.Warn("the server was started in a broad directory; only that directory is accepted as a project directory")
	}
	deps := tools.Deps{
		Models:    mcpModels{md: md},
		Playbooks: mcpPlaybooks{},
		Strain:    mcpStrain{now: time.Now},
		Applier:   mcpApplier{inner: spawnApplier{cat: cat, now: time.Now}, root: root, broad: broad},
		Harnesses: capsHarnessIDs,
		Version:   buildVersion,
	}
	srv, err := transport.New(transport.Info{Name: "downshift", Version: buildVersion, Instructions: mcpInstructions}, tools.Info(deps), log)
	if err != nil {
		fmt.Fprintln(errW, "mcp serve:", err)
		return 1
	}
	if err := srv.Serve(ctx, in, out); err != nil && ctx.Err() == nil {
		fmt.Fprintln(errW, "mcp serve:", err)
		return 1
	}
	return 0
}

// resolvedDir makes a directory absolute and follows symlinks, so a path under
// the project root can be compared with it. An unresolvable path is returned
// cleaned, and then matches nothing it should not.
func resolvedDir(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return filepath.Clean(dir)
	}
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		return real
	}
	return abs
}
