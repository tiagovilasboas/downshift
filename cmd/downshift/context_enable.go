// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/tiagovilasboas/downshift-ai/internal/contextopt"
)

func runContextEnable(ctx context.Context, reg *contextopt.Registry, args []string, w, errW io.Writer) int {
	providerID := "native"
	execFlag := ""
	var positional []string
	for _, a := range args {
		switch a {
		case "--exec", "--no-exec":
			execFlag = a
		default:
			positional = append(positional, a)
		}
	}
	if len(positional) > 0 {
		providerID = positional[0]
	}
	if execFlag != "" && providerID != "native" {
		fmt.Fprintf(errW, "error: %s applies to the native provider only\n", execFlag)
		return 2
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
	printExecEnabled(w, compactOn(cfg))
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
	// Native compact and model routing have no off switch: only a companion
	// record can be cleared.
	if len(args) == 0 {
		fmt.Fprintln(errW, "Native compact is always on and cannot be disabled. To clear a companion record: downshift context disable <companion>")
		return 2
	}
	cwd, _ := os.Getwd()
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
