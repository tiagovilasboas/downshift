// dsmon — downshift monitor
// Floating terminal widget. Lives in cmd/dsmon/ as a standalone command
// inside harness-downshift — reads events.jsonl, renders live in the terminal.
//
// Build:   go build -o dsmon ./cmd/dsmon
// Run:     ./dsmon
// Float:   ./cmd/dsmon/launch.sh   (opens a floating window)
//
// Zero external deps: pure stdlib, polling tail on events.jsonl.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// ── config ────────────────────────────────────────────────────────────────────

const (
	eventsFile = ".harness-downshift/events.jsonl"
	maxRecent  = 8
	refreshMs  = 250  // fast enough to feel live
	boxWidth   = 58
	window24h  = 24 * time.Hour
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

type event struct {
	Timestamp  string  `json:"timestamp"`
	Harness    string  `json:"harness"`
	Complexity string  `json:"complexity"`
	From       string  `json:"requested_model"`
	To         string  `json:"final_model"`
	Verdict    string  `json:"verdict"`
	Savings    float64 `json:"estimated_savings"`
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

// ── state (polling tail) ──────────────────────────────────────────────────────

type state struct {
	all      []event
	size     int64
	modTime  time.Time
	lastEvent time.Time // when the most recent event arrived
	tick     int        // frame counter for animations
}

func (s *state) reload(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	if fi, err := f.Stat(); err == nil {
		s.size, s.modTime = fi.Size(), fi.ModTime()
	}
	s.all = nil
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e event
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Harness != "" {
			s.all = append(s.all, e)
		}
	}
	if len(s.all) > 0 {
		s.lastEvent = time.Now()
	}
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
	f.Seek(s.size, 0) //nolint:errcheck
	added := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var e event
		if json.Unmarshal(sc.Bytes(), &e) == nil && e.Harness != "" {
			s.all = append(s.all, e)
			added++
		}
	}
	if added > 0 {
		s.lastEvent = time.Now()
	}
	s.size, s.modTime = fi.Size(), fi.ModTime()
	s.tick++
}

// ── stats ─────────────────────────────────────────────────────────────────────

type stats struct {
	total, down, up, ok int
	totalSavings        float64
	harnesses           []string // all distinct harnesses active today, most-recent first
}

func compute(all []event) (stats, []event) {
	var st stats
	var recent []event
	seen := map[string]bool{}
	for _, e := range all {
		if !e.isRecent() {
			continue
		}
		st.total++
		switch e.Verdict {
		case "DOWNSHIFT":
			st.down++
			st.totalSavings += e.Savings
		case "UPSHIFT":
			st.up++
		case "OK":
			st.ok++
		}
		if e.Harness != "" && !seen[e.Harness] {
			seen[e.Harness] = true
			st.harnesses = append(st.harnesses, e.Harness)
		}
		recent = append(recent, e)
	}
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
	bar := strings.Repeat("─", boxWidth-4-len(label)-1)
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
	from := shortModel(e.From)
	to := shortModel(e.To)
	comp := strings.ToLower(e.Complexity)
	if len(comp) > 7 {
		comp = comp[:7]
	}
	comp = comp + strings.Repeat(" ", 7-len(comp))
	switch e.Verdict {
	case "DOWNSHIFT":
		return fmt.Sprintf("  %s  %s%s%s→%s%s%s  %s%s%s  %s-%d%%%s",
			e.localTime(), dim, from, rst, grn, to, rst,
			dim, comp, rst, grn, int(e.Savings*100), rst)
	case "UPSHIFT":
		return fmt.Sprintf("  %s  %s%s%s→%s%s%s  %s%s%s  %s↑%s",
			e.localTime(), dim, from, rst, ylw, to, rst,
			dim, comp, rst, ylw, rst)
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

	// Estimated savings
	estUSD := st.totalSavings * 0.01
	estTokensK := int(estUSD / 0.000025)

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
	b.WriteString(divider("stats") + "\n")
	b.WriteString(row(fmt.Sprintf(
		"  %d events   %s%d↓%s  %s%d↑%s  %s%d✓%s",
		st.total, grn, st.down, rst, ylw, st.up, rst, dim, st.ok, rst,
	)) + "\n")
	if st.down > 0 {
		b.WriteString(row(fmt.Sprintf(
			"  est. saved  %s$%.2f%s  ~%dK tokens %s¹%s",
			grn, estUSD, rst, estTokensK, dim, rst,
		)) + "\n")
		b.WriteString(row(fmt.Sprintf(
			"  %s¹ estimated · actual tokens not yet tracked%s", dim, rst,
		)) + "\n")
	}
	b.WriteString(row("") + "\n")
	b.WriteString(bot + "\n")
	b.WriteString(fmt.Sprintf("%s  %s · ctrl+c to quit%s\n", dim, now, rst))

	fmt.Print(b.String())
}

// ── main ──────────────────────────────────────────────────────────────────────

func main() {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, eventsFile)

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
	render(s)

	for range time.NewTicker(refreshMs * time.Millisecond).C {
		s.poll(path)
		render(s)
	}
}
