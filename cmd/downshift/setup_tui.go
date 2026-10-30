// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
	"github.com/tiagovilasboas/downshift/internal/gateway/omniroute"
)

// setupTUI presents an interactive terminal UI wizard for onboarding and
// configuring AI coding harnesses with Downshift.
func setupTUI(o setupOptions) int {
	renderTUIHeader(o.Out)

	targets, code := setupSelect(o)
	if code != 0 {
		return code
	}
	if len(targets) == 0 {
		fmt.Fprintln(o.Out, "No supported AI harnesses detected on this machine.")
		fmt.Fprintln(o.Out, "Supported harnesses: Claude Code, Cursor, OpenCode, OpenAI Codex, Google Antigravity, KiroCrew (Experimental).")
		return 0
	}

	renderTUIHarnesses(o.Out, targets)

	if o.In == nil {
		o.Yes = true
		return setupRun(o)
	}

	reader := bufio.NewReader(o.In)

	// Step 1: Configure with the recommended defaults.
	fmt.Fprint(o.Out, "Step [1/2] • Configure the detected harnesses with the recommended defaults? [Y/n] (Y=yes, n=quit) ")
	ans, _ := reader.ReadString('\n')
	if isNo(ans) {
		fmt.Fprintln(o.Out, "Setup aborted by user. Nothing was written.")
		return 0
	}

	probeAndRenderGateways(o.Out)

	// Step 2: show the exact diff first (dry run), then ask. Default is No.
	preview := o
	preview.DryRun = true
	preview.Yes = false
	if code := setupRun(preview); code != 0 {
		return code
	}
	if o.DryRun {
		return 0
	}
	fmt.Fprint(o.Out, "Step [2/2] • Apply the changes above with an automatic backup? [y/N] ")
	confirm, _ := reader.ReadString('\n')
	if !isYes(confirm) {
		fmt.Fprintln(o.Out, "\nSetup aborted. Nothing was written.")
		return 0
	}

	o.Yes = true
	runCode := setupRun(o)
	if runCode == 0 {
		renderTUISmokeCheck(o.Out, targets)
		renderTUISuccess(o.Out)
	}
	return runCode
}

// isYes reports an explicit affirmative answer; empty or unknown is not one.
func isYes(ans string) bool {
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "s", "sim", "y", "yes":
		return true
	}
	return false
}

// isNo reports an explicit refusal.
func isNo(ans string) bool {
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "n", "nao", "não", "no":
		return true
	}
	return false
}

func renderTUIHeader(out io.Writer) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)
	fmt.Fprintln(out, titleStyle.Render("⚡ Downshift Setup Wizard — Intelligent Harness Onboarding"))
}

func renderTUIHarnesses(out io.Writer, targets []setupTarget) {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		MarginBottom(1)

	highlightStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("212")).
		Bold(true)

	subtleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	var sb strings.Builder
	sb.WriteString("AI harnesses detected on this machine (ordered by relevance):\n\n")
	for i, t := range targets {
		sb.WriteString(fmt.Sprintf("  [%d] %-14s ~/%s\n", i+1, highlightStyle.Render(t.Name), t.ConfigDir))
	}
	sb.WriteString("\n" + subtleStyle.Render("Downshift will apply model governance, token preservation and the shell compactor."))
	fmt.Fprintln(out, cardStyle.Render(sb.String()))
}

func probeAndRenderGateways(out io.Writer) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	omni := omniroute.New(150 * time.Millisecond)
	nine := ninerouter.New(150 * time.Millisecond)

	omniOK, _ := omni.CheckHealth(ctx, omni.DefaultEndpoint())
	nineOK, _ := nine.CheckHealth(ctx, nine.DefaultEndpoint())

	var detected []string
	if omniOK {
		detected = append(detected, fmt.Sprintf("OmniRoute (%s)", omni.DefaultEndpoint()))
	}
	if nineOK {
		detected = append(detected, fmt.Sprintf("9router (%s)", nine.DefaultEndpoint()))
	}

	if len(detected) > 0 {
		fmt.Fprintf(out, "✔ Active companion gateways detected: %s\n", strings.Join(detected, ", "))
	} else {
		fmt.Fprintln(out, "• No active local companion gateway (OmniRoute/9router) detected. Continuing.")
	}
	fmt.Fprintln(out)
}

func renderTUISuccess(out io.Writer) {
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(0, 1).
		MarginTop(1)

	boldStyle := lipgloss.NewStyle().Bold(true)
	msg := fmt.Sprintf("%s\n\n%s\n  • %s\n  • %s\n  • %s",
		boldStyle.Render("🎉 Setup completed successfully!"),
		"Next steps:",
		"Open a new window or session of your harness (Claude Code, Cursor, OpenCode, Codex, Antigravity, KiroCrew (Experimental)).",
		"Run 'downshift doctor' at any time to check the health of the connections.",
		"Run 'downshift stats' to follow your token savings in real time.",
	)
	fmt.Fprintln(out, cardStyle.Render(msg))
}
