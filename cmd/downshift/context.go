// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/benchmark"
	"github.com/tiagovilasboas/downshift/internal/compressor"
	"github.com/tiagovilasboas/downshift/internal/contextopt"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/headroom"
	"github.com/tiagovilasboas/downshift/internal/contextopt/providers/native"
	"github.com/tiagovilasboas/downshift/internal/sensor"
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
  disable [companion]  Disable context optimization, or clear one companion record
  metrics              Show estimated token reduction and command metrics from active provider
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
	fmt.Fprintf(w, "  Main Agent: %v, Subagents: %v\n", cfg.Scope.MainAgent, cfg.Scope.SpawnedAgents)
	fmt.Fprintf(w, "  Preserve Original Output: %v\n", cfg.Safety.PreserveOriginalOutput)

	p, ok := reg.Get(cfg.Provider)
	if ok {
		installed, ver, path, _ := p.Detect(ctx)
		switch {
		case installed && contextopt.IsCompanion(p):
			fmt.Fprintf(w, "  Binary: %s (v%s)\n", path, ver)
		case installed:
			fmt.Fprintf(w, "  Engine: %s (%s, zero dependencies)\n", path, ver)
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

func runContextEnable(ctx context.Context, reg *contextopt.Registry, args []string, w, errW io.Writer) int {
	providerID := "native"
	if len(args) > 0 {
		providerID = args[0]
	}
	p, ok := reg.Get(providerID)
	if !ok {
		fmt.Fprintf(errW, "error: unknown provider %q. Run `downshift context providers` to see list\n", providerID)
		return 2
	}

	if err := p.Validate(ctx); err != nil {
		fmt.Fprintf(errW, "error: provider %s validation failed: %v\n", providerID, err)
		return 1
	}

	cwd, _ := os.Getwd()
	if contextopt.IsCompanion(p) {
		return enableCompanion(ctx, p, cwd, w, errW)
	}
	var configured []string
	for _, h := range p.SupportedHarnesses() {
		if h.Supported {
			if err := p.Configure(ctx, cwd, h.HarnessID); err != nil {
				fmt.Fprintf(errW, "warning: configuring harness %s: %v\n", h.HarnessID, err)
			} else {
				configured = append(configured, h.HarnessID)
			}
		}
	}

	cfg, _ := contextopt.LoadConfig()
	cfg.Enabled = true
	cfg.Provider = providerID
	cfg.ActiveHarness = configured
	if providerID == "native" {
		cfg.ExitCodeContract = contextopt.ExitCodeContractObserve
	}
	if err := contextopt.SaveConfig(cfg); err != nil {
		fmt.Fprintf(errW, "error: failed to save config: %v\n", err)
		return 1
	}

	fmt.Fprintf(w, "Context optimization enabled. Provider %s is active.\n", p.Name())
	fmt.Fprintln(w, "Exit-code contract: observe_only; nonzero_exit_preserved; hook_cannot_replace_tool_output.")
	fmt.Fprintln(w, "Claude Code PostToolUse records CompressExit in observe mode when this flag is on, using the tool exit code.")
	fmt.Fprintln(w, "The hook cannot replace tool output. The model still sees the original result. Prompts are not compressed.")
	// A detected companion is recorded by default. Recording changes no hook
	// and runs no third-party command; it only tells doctor and the docs
	// that savings are not routing alone.
	for _, c := range companions(reg) {
		if installed, ver, _, _ := c.Detect(ctx); installed {
			if err := c.Configure(ctx, cwd, ""); err != nil {
				fmt.Fprintf(errW, "warning: recording companion %s: %v\n", c.ID(), err)
				continue
			}
			fmt.Fprintf(w, "Companion %s v%s detected and recorded. Wrap each harness yourself:\n", c.ID(), ver)
			printWrapHints(w, c)
		}
	}
	return 0
}

// enableCompanion records an external companion next to the in-process
// provider. Downshift never runs the companion's own setup.
func enableCompanion(ctx context.Context, p contextopt.Provider, cwd string, w, errW io.Writer) int {
	if err := p.Configure(ctx, cwd, ""); err != nil {
		fmt.Fprintf(errW, "error: recording companion %s: %v\n", p.ID(), err)
		return 1
	}
	fmt.Fprintf(w, "Companion %s recorded. The native provider stays active; hooks are unchanged.\n", p.ID())
	fmt.Fprintln(w, "Downshift does not launch or configure it. Wrap each harness yourself:")
	printWrapHints(w, p)
	return 0
}

func printWrapHints(w io.Writer, p contextopt.Provider) {
	for _, h := range p.SupportedHarnesses() {
		if h.Supported {
			fmt.Fprintf(w, "  %-12s %s\n", h.HarnessID, h.Notes)
		}
	}
}

func runContextDisable(ctx context.Context, reg *contextopt.Registry, args []string, w, errW io.Writer) int {
	cfg, err := contextopt.LoadConfig()
	if err != nil {
		fmt.Fprintf(errW, "error: loading config: %v\n", err)
		return 1
	}
	cwd, _ := os.Getwd()
	if len(args) > 0 {
		p, ok := reg.Get(args[0])
		if !ok || !contextopt.IsCompanion(p) {
			fmt.Fprintf(errW, "error: %q is not a registered companion. Run `downshift context providers`\n", args[0])
			return 2
		}
		if err := p.Disable(ctx, cwd, ""); err != nil {
			fmt.Fprintf(errW, "error: clearing companion %s: %v\n", p.ID(), err)
			return 1
		}
		fmt.Fprintf(w, "✓ Companion %s cleared. Unwrap your harness with its own command if it is still wrapped.\n", p.ID())
		return 0
	}
	if !cfg.Enabled && cfg.Companion == "" {
		fmt.Fprintln(w, "Context optimization is already disabled.")
		return 0
	}

	if p, ok := reg.Get(cfg.Provider); ok {
		for _, h := range cfg.ActiveHarness {
			_ = p.Disable(ctx, cwd, h)
		}
	}
	if c, ok := reg.Get(cfg.Companion); ok {
		_ = c.Disable(ctx, cwd, "")
	}

	cfg.Enabled = false
	cfg.Provider = ""
	cfg.Companion = ""
	cfg.ActiveHarness = nil
	if err := contextopt.SaveConfig(cfg); err != nil {
		fmt.Fprintf(errW, "error: saving config: %v\n", err)
		return 1
	}

	fmt.Fprintln(w, "✓ Context optimization disabled. Project configurations reverted.")
	return 0
}

func runContextMetrics(ctx context.Context, reg *contextopt.Registry, w, errW io.Writer) int {
	cfg, err := contextopt.LoadConfig()
	if err != nil || !cfg.Enabled {
		fmt.Fprintln(errW, "Context optimization is not enabled. Run `downshift context enable native` first.")
		return 1
	}
	p, ok := reg.Get(cfg.Provider)
	if !ok {
		fmt.Fprintf(errW, "Active provider %q is not registered.\n", cfg.Provider)
		return 1
	}

	cwd, _ := os.Getwd()
	metrics, err := p.GetMetrics(ctx, cwd)
	if err != nil {
		fmt.Fprintf(errW, "Failed to retrieve provider metrics: %v\n", err)
	}

	st, _ := sensor.DefaultStore()
	sensorObservations := map[string]sensor.SessionObservation{}
	if st != nil {
		sensorObservations, _ = st.GetSummary()
	}

	output := struct {
		ProviderMetrics    *contextopt.Metrics                  `json:"provider_metrics,omitempty"`
		SensorObservations map[string]sensor.SessionObservation `json:"context_sensor_observations,omitempty"`
	}{
		ProviderMetrics:    metrics,
		SensorObservations: sensorObservations,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(output)
	return 0
}

func runContextCompress(args []string, w, errW io.Writer) int {
	mode := compressor.ModeSafe
	exitCode := -1
	exitKnown := false
	var positional []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--exit" {
			if i+1 >= len(args) {
				fmt.Fprintln(errW, "context compress: --exit requires a status from 0 to 255")
				return 2
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n < 0 || n > 255 {
				fmt.Fprintln(errW, "context compress: --exit requires a status from 0 to 255")
				return 2
			}
			exitCode = n
			exitKnown = true
			i++
			continue
		}
		positional = append(positional, args[i])
	}
	if len(positional) > 0 {
		switch positional[0] {
		case "observe", "--observe":
			mode = compressor.ModeObserve
		case "safe", "--safe":
			mode = compressor.ModeSafe
		case "off", "--off":
			mode = compressor.ModeOff
		default:
			fmt.Fprintf(errW, "unknown compress mode %q (use observe, safe, or off)\n", positional[0])
			return 2
		}
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(errW, "error reading stdin: %v\n", err)
		return 1
	}

	if mode == compressor.ModeSafe && !exitKnown {
		fmt.Fprintln(errW, "context compress: safe mode requires --exit 0. Output preserved.")
		_, _ = w.Write(data)
		return 0
	}

	res := compressor.CompressExit(data, mode, exitCode)
	if mode == compressor.ModeObserve {
		fmt.Fprintf(errW, "context observe: format=%s original=%d bytes potential_reduced=%d bytes savings=%.1f%% (%s)\n",
			res.Format, res.OriginalBytes, res.ReducedBytes, res.SavingsRatio(), res.Reason)
		_, _ = w.Write(res.Output)
		return 0
	}

	if res.Applied {
		fmt.Fprintf(errW, "context compress: %s (reduced from %d to %d bytes, %.1f%% saved)\n",
			res.Reason, res.OriginalBytes, res.ReducedBytes, res.SavingsRatio())
	}
	_, _ = w.Write(res.Output)
	return 0
}
