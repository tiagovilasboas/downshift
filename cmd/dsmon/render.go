// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"time"
)

// ── ANSI ──────────────────────────────────────────────────────────────────────

const (
	curHome = "\033[H"
	clrScr  = "\033[2J"
	hideCur = "\033[?25l"
	showCur = "\033[?25h"
	rst     = "\033[0m"
	bold    = "\033[1m"
	dim     = "\033[2m"
	grn     = "\033[32m"
	ylw     = "\033[33m"
	cyn     = "\033[36m"
)

// ── rendering ─────────────────────────────────────────────────────────────────

func stripANSI(s string) string {
	var out []rune
	r := []rune(s)
	for i := 0; i < len(r); {
		if r[i] == '\033' && i+1 < len(r) && r[i+1] == '[' {
			i += 2
			for i < len(r) && r[i] != 'm' {
				i++
			}
			i++
			continue
		}
		out = append(out, r[i])
		i++
	}
	return string(out)
}

func row(content string) string {
	visible := []rune(stripANSI(content))
	pad := boxWidth - 2 - len(visible)
	if pad < 0 {
		pad = 0
	}
	return "│" + content + strings.Repeat(" ", pad) + "│"
}

func divider(label string) string {
	bar := strings.Repeat("─", boxWidth-4-len(label))
	return fmt.Sprintf("│ %s%s%s %s│", dim, label, rst, bar)
}

func shortModel(m string) string {
	switch {
	case strings.Contains(m, "haiku"):
		return "haiku  "
	case strings.Contains(m, "sonnet"):
		return "sonnet "
	case strings.Contains(m, "opus"):
		return "opus   "
	default:
		r := []rune(m)
		if len(r) > 7 {
			return string(r[:7])
		}
		return m + strings.Repeat(" ", 7-len(r))
	}
}

func switchLine(e event) string {
	from := shortModel(e.FromModel)
	to := shortModel(e.ToModel)
	comp := strings.ToLower(e.Complexity)
	if len(comp) > 7 {
		comp = comp[:7]
	}
	comp = comp + strings.Repeat(" ", 7-len(comp))
	switch e.Verdict {
	case "DOWNSHIFT":
		return fmt.Sprintf("  %s  %s%s%s→%s%s%s  %s%s%s  %s↓%s",
			e.localTime(), dim, from, rst, grn, to, rst,
			dim, comp, rst, grn, rst)
	case "UPSHIFT":
		return fmt.Sprintf("  %s  %s%s%s→%s%s%s  %s%s%s  %s↑%s",
			e.localTime(), dim, from, rst, ylw, to, rst,
			dim, comp, rst, ylw, rst)
	case "HELD":
		return fmt.Sprintf("  %s  %s%s%s  %s%s%s  %sheld%s",
			e.localTime(), cyn, from, rst, dim, comp, rst, dim, rst)
	default: // OK
		return fmt.Sprintf("  %s  %s%s%s  %s%s%s  %s✓%s",
			e.localTime(), cyn, from, rst, dim, comp, rst, grn, rst)
	}
}

func render(s *state) {
	st, recent := compute(s.all)

	// Build harness display: all distinct harnesses active today
	harnessDisplay := dim + "—" + rst
	if len(st.harnesses) > 0 {
		parts := make([]string, len(st.harnesses))
		for i, h := range st.harnesses {
			// highlight the most recent (last in slice)
			if i == len(st.harnesses)-1 {
				parts[i] = cyn + h + rst
			} else {
				parts[i] = dim + h + rst
			}
		}
		harnessDisplay = strings.Join(parts, dim+"·"+rst)
	}
	now := time.Now().Local().Format("15:04:05")

	// Animated live indicator: pulses ◉/○ every frame
	liveIcon := grn + "◉" + rst
	if s.tick%2 == 0 {
		liveIcon = dim + "○" + rst
	}

	// Last-event age counter — always ticking even when no new data
	lastEventStr := dim + "no events yet" + rst
	if !s.lastEvent.IsZero() {
		age := time.Since(s.lastEvent).Round(time.Second)
		switch {
		case age < 5*time.Second:
			lastEventStr = grn + "just now" + rst
		case age < 60*time.Second:
			lastEventStr = fmt.Sprintf("%s%ds ago%s", cyn, int(age.Seconds()), rst)
		case age < 3600*time.Second:
			lastEventStr = fmt.Sprintf("%s%dm ago%s", dim, int(age.Minutes()), rst)
		default:
			lastEventStr = fmt.Sprintf("%s%dh ago%s", dim, int(age.Hours()), rst)
		}
	}

	hr := strings.Repeat("─", boxWidth-2)
	top := bold + cyn + "╭" + hr + "╮" + rst
	bot := bold + cyn + "╰" + hr + "╯" + rst

	var b strings.Builder
	b.WriteString(curHome)
	b.WriteString(top + "\n")
	b.WriteString(row("") + "\n")
	b.WriteString(row(fmt.Sprintf("  %sdsmon%s  downshift monitor", bold, rst)) + "\n")
	b.WriteString(row("") + "\n")
	b.WriteString(row(fmt.Sprintf("  harnesses  %s  %s  last: %s",
		harnessDisplay, liveIcon, lastEventStr)) + "\n")
	b.WriteString(row("") + "\n")

	b.WriteString(divider("last 24h") + "\n")
	if len(recent) == 0 {
		b.WriteString(row(fmt.Sprintf("  %sno events yet%s", dim, rst)) + "\n")
	} else {
		for _, e := range recent {
			b.WriteString(row(switchLine(e)) + "\n")
		}
	}

	b.WriteString(row("") + "\n")

	// ── agents section (from agents.jsonl / dsmon-hook) ────────────────────────
	recentAgents := s.recentAgents()
	b.WriteString(divider("agents (24h)") + "\n")
	if len(recentAgents) == 0 {
		b.WriteString(row(fmt.Sprintf("  %sno spawns yet — hook: dsmon-agent-lifecycle%s", dim, rst)) + "\n")
	} else {
		for _, a := range recentAgents {
			model := a.Model
			if model == "" {
				model = "default"
			}
			if len([]rune(model)) > 6 {
				model = string([]rune(model)[:6])
			}
			task := a.Task
			if task == "" {
				// Privacy-safe line: no task text stored. Fall back to the
				// tool name so the row still identifies the spawn.
				if a.Tool != "" {
					task = a.Tool
				} else {
					task = "no task text (privacy)"
				}
			}
			if len([]rune(task)) > 28 {
				task = string([]rune(task)[:28]) + "…"
			}
			line := fmt.Sprintf("  %s  %s%-7s%s  %s%s%s",
				a.age(), cyn, model, rst, dim, task, rst)
			b.WriteString(row(line) + "\n")
		}
	}

	b.WriteString(row("") + "\n")
	b.WriteString(divider("stats") + "\n")
	b.WriteString(row(fmt.Sprintf(
		"  %d events   %s%d↓%s  %s%d↑%s  %s%d✓%s",
		st.total, grn, st.down, rst, ylw, st.up, rst, dim, st.ok, rst,
	)) + "\n")
	shown := false
	for _, h := range st.routing.HarnessOrder() {
		if line := st.routing.ByHarness[h].Line(); line != "" {
			b.WriteString(row(fmt.Sprintf("  %s%s: %s%s", grn, h, line, rst)) + "\n")
			shown = true
		}
	}
	if !shown && st.down > 0 {
		b.WriteString(row(fmt.Sprintf("  %stokens not observed yet%s", dim, rst)) + "\n")
	}
	b.WriteString(row("") + "\n")
	b.WriteString(bot + "\n")
	b.WriteString(fmt.Sprintf("%s  %s · ctrl+c to quit%s\n", dim, now, rst))

	fmt.Print(b.String())
}
