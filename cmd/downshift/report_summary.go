// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/accounting"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// paneRow is one harness at the head of the report: the figures the
// agents pane shows for a session (tokens saved, share of raw output cut,
// runs, hook against direct calls, spawn verdicts, share of subagent tokens
// on a cheaper model), here for the report's window and every session. Per
// harness, never summed; Compactor and routing tokens are never added.
type paneRow struct {
	Harness        string  `json:"harness"`
	TokensSaved    int64   `json:"compactor_tokens_saved"`
	RawBytes       int64   `json:"compactor_raw_bytes"`
	SavedBytes     int64   `json:"compactor_saved_bytes"`
	CutPct         float64 `json:"compactor_cut_pct"`
	Commands       int64   `json:"compactor_commands"`
	Compacted      int64   `json:"compactor_compacted"`
	RetrievedBytes int64   `json:"compactor_retrieved_bytes"`
	// HookCommands ran through the harness hook (session-tagged);
	// DirectCommands ran with no session (a typed `downshift compact`).
	HookCommands   int64 `json:"compactor_hook_commands"`
	DirectCommands int64 `json:"compactor_direct_commands"`
	ModelSwitched  int   `json:"spawns_model_switched"`
	EffortChanged  int   `json:"spawns_effort_changed"`
	Held           int   `json:"spawns_held"`
	HeldGuardrail  int   `json:"spawns_held_guardrail"`
	// CheaperPct is the share of subagent billable tokens that ran below the
	// session model; meaningful only when CheaperObserved.
	CheaperObserved bool    `json:"cheaper_observed"`
	CheaperPct      float64 `json:"cheaper_pct"`
}

// buildPaneRows computes each harness with events in scope from its own
// events only, through the same accounting.Compute every surface uses.
func buildPaneRows(events []telemetry.Event, execEvents []execwrap.Event, scope accounting.Scope) []paneRow {
	tel := map[string][]telemetry.Event{}
	for _, ev := range events {
		if ev.Harness != "" {
			tel[ev.Harness] = append(tel[ev.Harness], ev)
		}
	}
	exe := map[string][]execwrap.Event{}
	for _, ev := range execEvents {
		h := ev.Harness
		if h == "" {
			h = accounting.UnknownHarness
		}
		exe[h] = append(exe[h], ev)
	}
	out := []paneRow{}
	for _, h := range paneHarnesses(tel, exe) {
		if s, ok := paneRowOf(h, accounting.Compute(tel[h], exe[h], scope), accounting.DirectCompactor(exe[h], scope, time.Now())); ok {
			out = append(out, s)
		}
	}
	return out
}

func paneHarnesses(tel map[string][]telemetry.Event, exe map[string][]execwrap.Event) []string {
	seen := map[string]bool{}
	for h := range tel {
		seen[h] = true
	}
	for h := range exe {
		seen[h] = true
	}
	names := make([]string, 0, len(seen))
	for h := range seen {
		names = append(names, h)
	}
	sort.Strings(names)
	return names
}

// paneRowOf reports false for a harness with nothing in the window.
func paneRowOf(h string, n accounting.Numbers, direct accounting.Compactor) (paneRow, bool) {
	c := n.Compactor
	s := paneRow{
		Harness: h, TokensSaved: c.Tokens, RawBytes: c.RawBytes, SavedBytes: c.SavedBytes,
		Commands: c.Commands, Compacted: c.Compacted, RetrievedBytes: c.RetrievedBytes,
		HookCommands: c.Commands - direct.Commands, DirectCommands: direct.Commands,
		ModelSwitched: n.ModelSwitched, EffortChanged: n.EffortChanged, Held: n.Held, HeldGuardrail: n.HeldGuardrail,
	}
	if c.RawBytes > 0 {
		s.CutPct = 100 * float64(c.SavedBytes) / float64(c.RawBytes)
	}
	if r, ok := n.Routing.ByHarness[h]; ok && r.Spawns > 0 && r.Billable > 0 {
		s.CheaperObserved, s.CheaperPct = true, 100*float64(r.BelowBillable)/float64(r.Billable)
	}
	spawns := s.ModelSwitched + s.EffortChanged + s.Held
	return s, c.Commands > 0 || spawns > 0 || s.CheaperObserved
}

// paneCells is one harness's row, shared by the markdown and text report.
func paneCells(s paneRow) []string {
	cut := "-"
	if s.RawBytes > 0 {
		cut = fmt.Sprintf("%.0f%% (%s of %s)", s.CutPct, humanBytes(s.SavedBytes), humanBytes(s.RawBytes))
	}
	cheaper := "unobserved"
	if s.CheaperObserved {
		cheaper = fmt.Sprintf("%.0f%%", s.CheaperPct)
	}
	read := "-"
	if s.RetrievedBytes > 0 {
		read = humanBytes(s.RetrievedBytes)
	}
	return []string{
		s.Harness, telemetry.ShortTokens(s.TokensSaved), cut,
		fmt.Sprintf("%d · %d", s.Commands, s.Compacted), fmt.Sprintf("%d · %d", s.HookCommands, s.DirectCommands), read,
		fmt.Sprintf("%d · %d · %d (%d guardrail)", s.ModelSwitched, s.EffortChanged, s.Held, s.HeldGuardrail), cheaper,
	}
}

var paneHeader = []string{
	"harness", "tokens saved", "output cut", "runs · compacted", "hook · direct", "read back",
	"subagents: model · effort · held", "subagent tokens on a cheaper model",
}

// summaryNote says what the table covers and what it never does.
const summaryNote = "Per harness, never summed. Tokens saved are the Compactor's (saved bytes / 4, an estimate, " +
	"net of what `retrieve` read back). Cheaper-model tokens still ran, below the session model: never added to tokens saved."

func printSummaryMarkdown(w io.Writer, rows []paneRow) {
	fmt.Fprint(w, "## Compaction savings per harness\n\nTokens kept out of the context by the Compactor, with the rest of what the agents pane shows, per harness.\n\n")
	if len(rows) == 0 {
		fmt.Fprintf(w, "No compaction events in this window.\n\n%s\n", noEventsLine(nil))
		return
	}
	fmt.Fprint(w, "|")
	for _, h := range paneHeader {
		fmt.Fprintf(w, " %s |", h)
	}
	fmt.Fprint(w, "\n|---|--:|--:|--:|--:|--:|--:|--:|\n")
	for _, s := range rows {
		fmt.Fprint(w, "|")
		for _, c := range paneCells(s) {
			fmt.Fprintf(w, " %s |", c)
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "\n_%s_\n\n%s\n", summaryNote, noEventsLine(paneNames(rows)))
}

// paneNames is the set of harnesses with a row, for the no-events line.
func paneNames(rows []paneRow) map[string]bool {
	have := map[string]bool{}
	for _, r := range rows {
		have[r.Harness] = true
	}
	return have
}

func printSummaryText(w io.Writer, rows []paneRow, scope string) {
	fmt.Fprintf(w, "COMPACTION SAVINGS PER HARNESS (%s; tokens kept out of the context, per harness, never summed)\n", scope)
	if len(rows) == 0 {
		fmt.Fprintln(w, "   No compaction events in this window.")
		if rest := harnessesWithoutEvents(nil); len(rest) > 0 {
			fmt.Fprintf(w, "   no events: %s\n", strings.Join(rest, ", "))
		}
		fmt.Fprintln(w)
		return
	}
	for _, s := range rows {
		cells := paneCells(s)
		fmt.Fprintf(w, "   %-14s %s tokens saved\n", cells[0], cells[1])
		for i := 2; i < len(cells); i++ {
			fmt.Fprintf(w, "      %-36s %s\n", paneHeader[i], cells[i])
		}
	}
	fmt.Fprintf(w, "   %s\n", summaryNote)
	if rest := harnessesWithoutEvents(paneNames(rows)); len(rest) > 0 {
		fmt.Fprintf(w, "   no events: %s\n", strings.Join(rest, ", "))
	}
	fmt.Fprintln(w)
}
