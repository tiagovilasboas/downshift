// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/benchmark"
	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/headroom"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/native"
)

// contextRegistry builds the registry for `downshift context`; tests swap it.
var contextRegistry = defaultContextRegistry

func defaultContextRegistry() *contextopt.Registry {
	reg := contextopt.NewRegistry()
	reg.Register(native.NewProvider(compressor.ModeObserve))
	reg.Register(headroom.NewProvider())
	return reg
}

// companions lists the registered external companions in a stable order.
func companions(reg *contextopt.Registry) []contextopt.Provider {
	var out []contextopt.Provider
	for _, p := range reg.List() {
		if contextopt.IsCompanion(p) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

func runContext(args []string, w, errW io.Writer) int {
	if len(args) == 0 {
		printContextUsage(errW)
		return 2
	}

	reg := contextRegistry()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	switch args[0] {
	case "status":
		return runContextStatus(ctx, reg, w, errW)
	case "providers":
		return runContextProviders(reg, w)
	case "doctor":
		return runContextDoctor(ctx, reg, w, errW)
	case "enable":
		return runContextEnable(ctx, reg, args[1:], w, errW)
	case "disable":
		return runContextDisable(ctx, reg, args[1:], w, errW)
	case "metrics":
		return runContextMetrics(ctx, reg, w, errW)
	case "compress":
		return runContextCompress(args[1:], w, errW)
	case "benchmark":
		if err := benchmark.CompareContextScenarios(w); err != nil {
			fmt.Fprintf(errW, "benchmark error: %v\n", err)
			return 1
		}
		return 0
	case "-h", "--help", "help":
		printContextUsage(w)
		return 0
	default:
		fmt.Fprintf(errW, "unknown context command %q\n\n", args[0])
		printContextUsage(errW)
		return 2
	}
}

func printContextUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: downshift context <subcommand>

Subcommands:
  status               Show current context optimization status and active provider
  providers            List supported context optimization companion providers
  doctor               Run diagnostics on context optimization providers and harnesses
  enable [provider]    Enable the native provider (default) or record a companion (headroom)
  disable [companion]  Clear one companion record. Native compact is always on and cannot be disabled
  metrics              Show command metrics, and bytes saved by downshift compact (tokens estimated)
  compress [mode]      Compress stdin tool output (mode: observe, safe, off)
  benchmark            Print the scenario matrix and measured compressor bytes`)
}

func runContextStatus(ctx context.Context, reg *contextopt.Registry, w, errW io.Writer) int {
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		fmt.Fprintf(errW, "context status: failed to load config: %v\n", err)
		return 1
	}

	fmt.Fprintln(w, "Context Optimization:")
	if !cfg.Enabled {
		fmt.Fprintln(w, "  Status: Disabled")
		fmt.Fprintln(w, "  Hint: Run `downshift context enable` to enable Native Context Compressor")
		return 0
	}

	fmt.Fprintf(w, "  Status: Enabled\n")
	fmt.Fprintf(w, "  Active Provider: %s\n", cfg.Provider)
	if len(cfg.ActiveHarness) > 0 {
		fmt.Fprintf(w, "  Active Harnesses: %s\n", strings.Join(cfg.ActiveHarness, ", "))
	}
	if cfg.Provider == "native" {
		fmt.Fprintln(w, "  Claude Code: measures only. A Claude Code hook cannot rewrite tool output already produced, so the PostToolUse path saves no tokens;")
		fmt.Fprintln(w, "    Compact (below) shortens output by rewriting the command before it runs. Other compression needs the Headroom proxy (`headroom wrap claude`), which Downshift does not start.")
	}
	if compactOn(cfg) {
		fmt.Fprintln(w, "  Compact: on (allowlisted shell commands run through `downshift compact`)")
	} else {
		fmt.Fprintln(w, "  Compact: off (\"exec\": false in the config, or DOWNSHIFT_EXEC=0)")
	}
	fmt.Fprintf(w, "  Main Agent: %v, Subagents: %v\n", cfg.Scope.MainAgent, cfg.Scope.SpawnedAgents)
	fmt.Fprintf(w, "  Preserve Original Output: %v\n", cfg.Safety.PreserveOriginalOutput)

	p, ok := reg.Get(cfg.Provider)
	if ok {
		installed, ver, path, _ := p.Detect(ctx)
		switch {
		case installed && contextopt.IsCompanion(p):
			fmt.Fprintf(w, "  Binary: %s (v%s)\n", path, ver)
		case installed:
			fmt.Fprintf(w, "  Engine: %s (%s, standard library only)\n", path, ver)
		default:
			fmt.Fprintln(w, "  ⚠️  Configured provider binary is not found on PATH!")
		}
	}
	for _, c := range companions(reg) {
		installed, ver, _, _ := c.Detect(ctx)
		switch {
		case cfg.Companion == c.ID() && installed:
			fmt.Fprintf(w, "  Companion: %s (v%s, recorded as active; wrap each harness with its own command)\n", c.ID(), ver)
		case cfg.Companion == c.ID():
			fmt.Fprintf(w, "  Companion: %s recorded but not on PATH (`downshift context disable %s` to clear)\n", c.ID(), c.ID())
		case installed:
			fmt.Fprintf(w, "  Companion available: %s v%s detected, not recorded (`downshift context enable %s`)\n", c.ID(), ver, c.ID())
		}
	}
	return 0
}

func runContextProviders(reg *contextopt.Registry, w io.Writer) int {
	fmt.Fprintln(w, "Supported Context Optimization Providers:")
	for _, p := range reg.List() {
		fmt.Fprintf(w, "\n• %s (id: %s)\n", p.Name(), p.ID())
		fmt.Fprintln(w, "  Capabilities:")
		for _, c := range p.Capabilities() {
			fmt.Fprintf(w, "    - %s\n", c)
		}
		fmt.Fprintln(w, "  Supported Harnesses:")
		for _, h := range p.SupportedHarnesses() {
			status := "supported"
			if !h.Supported {
				status = "planned/unverified"
			}
			fmt.Fprintf(w, "    - %-12s [%s via %s] %s\n", h.HarnessID, status, h.Method, h.Notes)
		}
	}
	return 0
}

func runContextDoctor(ctx context.Context, reg *contextopt.Registry, w, errW io.Writer) int {
	cwd, _ := os.Getwd()
	fmt.Fprintln(w, "Context Optimization Doctor:")
	for _, p := range reg.List() {
		diag, err := p.GetDiagnostics(ctx, cwd)
		if err != nil {
			fmt.Fprintf(errW, "  Provider %s: diagnostic error: %v\n", p.ID(), err)
			continue
		}
		fmt.Fprintf(w, "\nProvider: %s\n", p.Name())
		if diag.Installed {
			fmt.Fprintf(w, "  ✓ Installed: %s (v%s)\n", diag.BinaryPath, diag.Version)
		} else {
			fmt.Fprintf(w, "  ✗ Installed: No (binary not found on PATH)\n")
		}
		if len(diag.ActiveHarnesses) > 0 {
			fmt.Fprintf(w, "  Active local configurations: %s\n", strings.Join(diag.ActiveHarnesses, ", "))
		}
		if len(diag.Issues) > 0 {
			fmt.Fprintln(w, "  Issues:")
			for _, iss := range diag.Issues {
				fmt.Fprintf(w, "    - %s\n", iss)
			}
		}
		if len(diag.Recommendations) > 0 {
			fmt.Fprintln(w, "  Recommendations:")
			for _, rec := range diag.Recommendations {
				fmt.Fprintf(w, "    - %s\n", rec)
			}
		}
	}
	return 0
}
