// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// dsmon — downshift monitor
// Floating terminal widget. Lives in cmd/dsmon/ as a standalone command
// inside Downshift — reads events.jsonl, renders live in the terminal.
//
// Build:   go build -o dsmon ./cmd/dsmon
// Run:     ./dsmon
// Float:   ./cmd/dsmon/launch.sh   (opens a floating window)
//
// Standard library only: polling tail on events.jsonl.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

// ── config ────────────────────────────────────────────────────────────────────

const (
	maxRecent = 8
	refreshMs = 250 // fast enough to feel live
	boxWidth  = 58
	window24h = 24 * time.Hour
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

// ── event ─────────────────────────────────────────────────────────────────────

// event is one event-log line. It reuses the telemetry schema instead of
// keeping a private copy of it.
type event struct {
	telemetry.Event
}

func (e event) localTime() string {
	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	if err != nil || len(e.Timestamp) < 16 {
		return "??:??"
	}
	// Show relative time for recent events so the list feels live
	age := time.Since(t)
	switch {
	case age < 60*time.Second:
		return fmt.Sprintf("%2ds", int(age.Seconds()))
	case age < 3600*time.Second:
		return fmt.Sprintf("%2dm", int(age.Minutes()))
	default:
		return t.Local().Format("15:04")
	}
}

// isRecent returns true for events within the last 24 hours.
// Using 24h instead of "today" avoids timezone edge cases and midnight cutoffs.
func (e event) isRecent() bool {
	t, err := time.Parse(time.RFC3339Nano, e.Timestamp)
	if err != nil {
		return false
	}
	return time.Since(t) < window24h
}

// ── agent event (agents.jsonl from dsmon-hook) ────────────────────────────────

type agentEvent struct {
	Timestamp string `json:"timestamp"`
	Tool      string `json:"tool"`
	Session   string `json:"session"`
	Task      string `json:"task"`
	Model     string `json:"model"`
}

func (a agentEvent) isRecent() bool {
	t, err := time.Parse(time.RFC3339Nano, a.Timestamp)
	return err == nil && time.Since(t) < window24h
}

func (a agentEvent) age() string {
	t, err := time.Parse(time.RFC3339Nano, a.Timestamp)
	if err != nil {
		return "?"
	}
	d := time.Since(t)
	switch {
	case d < 60*time.Second:
		return fmt.Sprintf("%2ds", int(d.Seconds()))
	case d < 3600*time.Second:
		return fmt.Sprintf("%2dm", int(d.Minutes()))
	default:
		return t.Local().Format("15:04")
	}
}

// ── state (polling tail) ──────────────────────────────────────────────────────

const (
	// maxKeptEvents and maxKeptAgents bound memory: the widget only shows the
	// last 24 hours and a handful of rows, so older lines are dropped.
	maxKeptEvents = 5000
	maxKeptAgents = 500
)

type state struct {
	all       []event
	agents    []agentEvent
	size      int64
	modTime   time.Time
	agentSize int64
	lastEvent time.Time
	tick      int
}

// readNewLines reads complete lines from f starting at offset up to EOF and
// returns the offset after the last complete line it consumed. A trailing
// line without a newline is left for the next poll (the writer may be
// mid-append), and a line over telemetry.MaxLineBytes is skipped.
func readNewLines(f *os.File, offset int64, handle func(line []byte)) (int64, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return offset, err
	}
	r := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			// io.EOF: line (if any) is an unterminated partial; do not consume it.
			if err == io.EOF {
				return offset, nil
			}
			return offset, err
		}
		offset += int64(len(line))
		if len(line) > telemetry.MaxLineBytes {
			continue
		}
		handle(line)
	}
}

func (s *state) addEvent(line []byte) bool {
	var e event
	if json.Unmarshal(line, &e) != nil || e.Harness == "" {
		return false
	}
	s.all = append(s.all, e)
	return true
}

// prune keeps only the recent window and at most maxKeptEvents events.
func (s *state) prune() {
	kept := s.all[:0]
	for _, e := range s.all {
		if e.isRecent() {
			kept = append(kept, e)
		}
	}
	if len(kept) > maxKeptEvents {
		kept = append([]event(nil), kept[len(kept)-maxKeptEvents:]...)
	}
	s.all = kept
}

func (s *state) reload(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return
	}
	s.all, s.size, s.modTime = nil, 0, fi.ModTime()
	n, _ := readNewLines(f, 0, func(l []byte) { s.addEvent(l); s.prune1() })
	s.size = n
	if len(s.all) > 0 {
		s.lastEvent = time.Now()
	}
}

// prune1 bounds growth while a large file is read: it only trims when the
// slice has doubled past the cap, so the cost stays amortised.
func (s *state) prune1() {
	if len(s.all) > 2*maxKeptEvents {
		s.prune()
	}
}

func (s *state) pollAgents(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() == s.agentSize {
		return
	}
	if fi.Size() < s.agentSize {
		// file rotated — reload from scratch
		s.agents = nil
		s.agentSize = 0
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	n, _ := readNewLines(f, s.agentSize, func(l []byte) {
		var ae agentEvent
		// Keep privacy-safe lines: new hook events have an empty task but
		// always carry a tool name. Skip only lines with neither.
		if json.Unmarshal(l, &ae) == nil && (ae.Task != "" || ae.Tool != "") {
			s.agents = append(s.agents, ae)
		}
	})
	s.agentSize = n
	if len(s.agents) > maxKeptAgents {
		s.agents = append([]agentEvent(nil), s.agents[len(s.agents)-maxKeptAgents:]...)
	}
}

func (s *state) recentAgents() []agentEvent {
	var out []agentEvent
	for _, a := range s.agents {
		if a.isRecent() {
			out = append(out, a)
		}
	}
	// last 6
	if len(out) > 6 {
		out = out[len(out)-6:]
	}
	return out
}

func (s *state) poll(path string) {
	fi, err := os.Stat(path)
	if err != nil || (fi.Size() == s.size && fi.ModTime().Equal(s.modTime)) {
		s.tick++
		return
	}
	if fi.Size() < s.size {
		s.reload(path)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	added := 0
	n, _ := readNewLines(f, s.size, func(l []byte) {
		if s.addEvent(l) {
			added++
		}
	})
	if added > 0 {
		s.lastEvent = time.Now()
		s.prune()
	}
	// Advance by the bytes actually consumed, not the size seen by Stat:
	// lines appended between Stat and read are picked up next poll, never
	// read twice or skipped.
	s.size, s.modTime = n, fi.ModTime()
	s.tick++
}

// ── stats ─────────────────────────────────────────────────────────────────────

type stats struct {
	total, down, up, ok int
	// held counts classified shifts the hook left unchanged.
	held int
	// tokens is the shared tokens-saved view (telemetry.TokenSavings): a live
	// widget shows tokens, never dollars (`downshift report` prices them).
	tokens    telemetry.TokenSaving
	harnesses []string // all distinct harnesses active today, most-recent first
}

func compute(all []event) (stats, []event) {
	var st stats
	var recent []event
	var window []telemetry.Event
	seen := map[string]bool{}
	for _, e := range all {
		if !e.isRecent() {
			continue
		}
		window = append(window, e.Event)
		if telemetry.IsCostOnlyOutcome(e.Outcome) {
			// Usage records carry real cost (summed by Summarize below);
			// baseline records are the no-route control group. Neither is
			// a routing decision.
			continue
		}
		st.total++
		applied := telemetry.AppliedRewrite(e.Outcome, e.Corrections)
		switch {
		case e.Verdict == "DOWNSHIFT" && applied:
			st.down++
		case e.Verdict == "UPSHIFT" && applied:
			st.up++
		case e.Verdict == "DOWNSHIFT" || e.Verdict == "UPSHIFT":
			st.held++
			e.Verdict = "HELD" // displayed as unchanged, never as a switch
		case e.Verdict == "OK":
			st.ok++
		}
		if e.Harness != "" && !seen[e.Harness] {
			seen[e.Harness] = true
			st.harnesses = append(st.harnesses, e.Harness)
		}
		recent = append(recent, e)
	}
	st.tokens = telemetry.TokenSavings(window).Total
	if len(recent) > maxRecent {
		recent = recent[len(recent)-maxRecent:]
	}
	return st, recent
}

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
	if line := st.tokens.Line(); line != "" {
		b.WriteString(row(fmt.Sprintf("  %s%s%s", grn, line, rst)) + "\n")
	} else if st.down > 0 {
		b.WriteString(row(fmt.Sprintf("  %stokens not observed yet%s", dim, rst)) + "\n")
	}
	b.WriteString(row("") + "\n")
	b.WriteString(bot + "\n")
	b.WriteString(fmt.Sprintf("%s  %s · ctrl+c to quit%s\n", dim, now, rst))

	fmt.Print(b.String())
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	fromcore.Install(catalog.Load())
	home, _ := os.UserHomeDir()
	path, err := paths.EventsPath()
	if err != nil {
		path = filepath.Join(home, paths.DirName, "events.jsonl")
	}
	agentPath, err := paths.AgentsPath()
	if err != nil {
		agentPath = filepath.Join(home, paths.DirName, "agents.jsonl")
	}

	fmt.Print(hideCur)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		fmt.Print(showCur + "\n")
		os.Exit(0)
	}()

	fmt.Print(clrScr)
	s := &state{}
	s.reload(path)
	s.pollAgents(agentPath)
	render(s)

	for range time.NewTicker(refreshMs * time.Millisecond).C {
		s.poll(path)
		s.pollAgents(agentPath)
		render(s)
	}
}
