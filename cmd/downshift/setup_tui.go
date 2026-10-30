// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// setupTUI presents an interactive terminal UI for onboarding and configuring
// AI coding harnesses with Downshift.
func setupTUI(o setupOptions) int {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		MarginBottom(1)

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

	fmt.Fprintln(o.Out, titleStyle.Render("⚡ Downshift Setup — Interactive Harness Onboarding"))

	targets, code := setupSelect(o)
	if code != 0 {
		return code
	}
	if len(targets) == 0 {
		fmt.Fprintln(o.Out, "No supported AI harnesses detected on this machine.")
		fmt.Fprintln(o.Out, "Supported harnesses: Claude Code, Cursor, OpenCode, OpenAI Codex, Google Antigravity, KiroCrew.")
		return 0
	}

	var sb strings.Builder
	sb.WriteString("Detected AI Coding Harnesses on this host:\n\n")
	for i, t := range targets {
		sb.WriteString(fmt.Sprintf("  [%d] %-14s ~/%s\n", i+1, highlightStyle.Render(t.Name), t.ConfigDir))
	}
	sb.WriteString("\n" + subtleStyle.Render("Downshift will apply your model choices within your policy and compact terminal output."))

	fmt.Fprintln(o.Out, cardStyle.Render(sb.String()))

	if o.In == nil {
		// Non-interactive fallback
		o.Yes = true
		return setupRun(o)
	}

	reader := bufio.NewReader(o.In)
	fmt.Fprint(o.Out, "Configure all detected harnesses with recommended zero-prompt defaults? [Y/n] ")
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))

	if ans != "" && ans != "y" && ans != "yes" {
		fmt.Fprintln(o.Out, "Setup aborted by user.")
		return 0
	}

	o.Yes = true
	return setupRun(o)
}
