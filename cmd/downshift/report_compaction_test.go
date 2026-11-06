// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// seedCompactionTwoHarnesses records exec compaction events for two harnesses
// so the leading section has more than one row: claude-code saves 4000 bytes
// (1000 tokens) and codex saves 800 bytes (200 tokens).
func seedCompactionTwoHarnesses(t *testing.T) {
	t.Helper()
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, ev := range []execwrap.Event{
		{TS: now, Harness: "claude-code", RawBytes: 4400, EmittedBytes: 400, Compacted: true},
		{TS: now, Harness: "codex", RawBytes: 1000, EmittedBytes: 200, Compacted: true},
	} {
		if err := execwrap.Append(path, ev); err != nil {
			t.Fatal(err)
		}
	}
}

// The report opens with the Compactor's savings per harness, in tokens, before
// any routing section.
func TestReport_LeadsWithCompactionPerHarness(t *testing.T) {
	whyFixtureEnv(t)
	seedCompactionTwoHarnesses(t)
	text := runReportOut(t, "--days=0", "--text")

	lead := strings.Index(text, "COMPACTION SAVINGS PER HARNESS")
	if lead < 0 {
		t.Fatalf("report has no leading compaction section:\n%s", text)
	}
	// The leading section comes before the routing comparison and the routing
	// note, whichever heading routing uses.
	for _, after := range []string{"Routed spawns", "Current harness:", "WHY SPAWNS CHANGED", "QUOTA USED PER HARNESS"} {
		if i := strings.Index(text, after); i >= 0 && i < lead {
			t.Errorf("%q (at %d) comes before the compaction lead (at %d):\n%s", after, i, lead, text)
		}
	}

	// It is per harness, in tokens: claude-code 1000, codex 200. The two never
	// share a line and are never summed into 1200.
	section := text[lead:]
	if i := strings.Index(section, "\nRouted spawns"); i >= 0 {
		section = section[:i]
	}
	if i := strings.Index(section, "\nCurrent harness:"); i >= 0 {
		section = section[:i]
	}
	var claude, codex string
	for _, line := range strings.Split(section, "\n") {
		switch {
		case strings.Contains(line, "claude-code"):
			claude = line
		case strings.Contains(line, "codex"):
			codex = line
		}
	}
	if !strings.Contains(claude, telemetry.ShortTokens(1000)) {
		t.Errorf("claude-code lead row lacks %q: %q", telemetry.ShortTokens(1000), claude)
	}
	if !strings.Contains(codex, telemetry.ShortTokens(200)) {
		t.Errorf("codex lead row lacks %q: %q", telemetry.ShortTokens(200), codex)
	}
	if strings.Contains(section, telemetry.ShortTokens(1200)) {
		t.Errorf("per-harness token savings must not be summed:\n%s", section)
	}
}

// The leading section names tokens, never money.
func TestReport_CompactionLeadHasNoMoney(t *testing.T) {
	whyFixtureEnv(t)
	seedCompactionTwoHarnesses(t)
	text := runReportOut(t, "--days=0", "--text")
	lead := strings.Index(text, "COMPACTION SAVINGS PER HARNESS")
	if lead < 0 {
		t.Fatalf("report has no leading compaction section:\n%s", text)
	}
	section := text[lead:]
	if i := strings.Index(section, "Routed spawns"); i >= 0 {
		section = section[:i]
	}
	if i := strings.Index(section, "Current harness:"); i >= 0 {
		section = section[:i]
	}
	lower := strings.ToLower(section)
	// Money is banned; "savings" in tokens is the roadmap's own wording for
	// this section (P2.2), so it is allowed as long as no currency appears.
	for _, banned := range []string{"$", "cost", "price", "usd", "dollar"} {
		if strings.Contains(lower, banned) {
			t.Errorf("the compaction lead mentions %q:\n%s", banned, section)
		}
	}
}

// An empty log still prints the leading section with a plain sentence, and it
// is still the first section.
func TestReport_CompactionLeadWithEmptyLog(t *testing.T) {
	endEnv(t)
	text := runReportOut(t, "--text")
	lead := strings.Index(text, "COMPACTION SAVINGS PER HARNESS")
	if lead < 0 {
		t.Fatalf("empty report has no leading compaction section:\n%s", text)
	}
	for _, after := range []string{"Routed spawns", "WHY SPAWNS CHANGED", "QUOTA USED PER HARNESS"} {
		if i := strings.Index(text, after); i >= 0 && i < lead {
			t.Errorf("%q comes before the compaction lead on an empty log:\n%s", after, text)
		}
	}
	if !strings.Contains(text[lead:], "No compaction events in this window.") {
		t.Errorf("empty compaction lead lacks its plain sentence:\n%s", text[lead:])
	}
}

// The --json report keeps a machine-readable per-harness compaction field with
// no money and no summed total.
func TestReport_JSONCarriesCompactionLead(t *testing.T) {
	whyFixtureEnv(t)
	seedCompactionTwoHarnesses(t)
	var rep struct {
		Compaction []struct {
			Harness          string `json:"harness"`
			CompactorAvoided int64  `json:"compactor_tokens_avoided"`
		} `json:"compaction_by_harness"`
	}
	if err := json.Unmarshal([]byte(runReportOut(t, "--days=0", "--json")), &rep); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, r := range rep.Compaction {
		got[r.Harness] = r.CompactorAvoided
	}
	if got["claude-code"] != 1000 || got["codex"] != 200 {
		t.Errorf("compaction_by_harness = %v, want claude-code 1000 and codex 200", got)
	}
}
