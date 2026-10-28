// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/accounting"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// whyFixtureEnv isolates HOME, the state dir, the exec log and the quota file,
// and points the event log at a copy of testdata/report_why/events.jsonl.
func whyFixtureEnv(t *testing.T) {
	t.Helper()
	endEnv(t)
	t.Setenv(paths.EnvStateDir, t.TempDir())
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	raw, err := os.ReadFile(filepath.Join("testdata", "report_why", "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(log, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
}

func runReportOut(t *testing.T, args ...string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if rc := runReport(args, &out, &errOut); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errOut.String())
	}
	return out.String()
}

func seedCompactorAndQuota(t *testing.T) {
	t.Helper()
	path, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	ev := execwrap.Event{TS: time.Now().UTC().Format(time.RFC3339), Harness: "claude-code", RawBytes: 4400, EmittedBytes: 400, Compacted: true}
	if err := execwrap.Append(path, ev); err != nil {
		t.Fatal(err)
	}
	five, week := 42.0, 17.0
	now := time.Now().UTC()
	snap := quota.Snapshot{Version: 1, Harness: "claude-code", Source: "claude-statusline", ObservedAt: now, ExpiresAt: now.Add(10 * time.Minute),
		Windows: []quota.Window{
			{ID: "five_hour", Scope: "harness", UsedPercent: &five},
			{ID: "seven_day", Scope: "harness", UsedPercent: &week},
		}}
	qp, err := quota.Path()
	if err != nil {
		t.Fatal(err)
	}
	if err := quota.Store(qp, snap); err != nil {
		t.Fatal(err)
	}
}

// sectionOf returns the text from heading to the final-estimate line.
func sectionOf(t *testing.T, text, heading string) string {
	t.Helper()
	i := strings.Index(text, heading)
	if i < 0 {
		t.Fatalf("report lacks %q:\n%s", heading, text)
	}
	return text[i:]
}

func TestReport_WhySpawnsChangedPerHarness(t *testing.T) {
	whyFixtureEnv(t)
	text := runReportOut(t, "--days=0")
	why := sectionOf(t, text, "WHY SPAWNS CHANGED")
	why = why[:strings.Index(why, "TOKENS PER HARNESS")]
	for _, want := range []string{
		"claude-code: 6 spawns",
		"by verdict: ", "kept 1", "rule 1", "profile 1", "hint_down 1", "refused_config 1",
		"rule:fast-lane 1", "profile:cheap-reader 1", "refused_config:unknown_profile 1",
		"1 without a routing note",
		"codex: 2 spawns", "held_unknown:no_candidate 1",
		"no events: antigravity, cursor, kirocrew, opencode",
	} {
		if !strings.Contains(why, want) {
			t.Errorf("why section lacks %q:\n%s", want, why)
		}
	}
	if strings.Contains(why, "cursor: ") {
		t.Errorf("a harness without events must not get its own block:\n%s", why)
	}
}

func TestReport_TokensPerHarnessKeepsColumnsApart(t *testing.T) {
	whyFixtureEnv(t)
	seedCompactorAndQuota(t)
	text := runReportOut(t, "--days=0")
	tab := sectionOf(t, text, "TOKENS PER HARNESS")
	tab = tab[:strings.Index(tab, "\nTokens saved (estimate)")]
	var claude, codex string
	for _, line := range strings.Split(tab, "\n") {
		switch {
		case strings.Contains(line, "claude-code"):
			claude = line
		case strings.Contains(line, "codex"):
			codex = line
		}
	}
	// 4000 saved bytes is 1000 tokens avoided; 1500 child tokens moved to haiku.
	for _, want := range []string{telemetry.ShortTokens(1000), telemetry.ShortTokens(1500), "42% · 17%"} {
		if !strings.Contains(claude, want) {
			t.Errorf("claude-code row lacks %q: %q", want, claude)
		}
	}
	if !strings.Contains(codex, "unobserved") || !strings.Contains(codex, "unknown") {
		t.Errorf("codex row must read unobserved and unknown, got %q", codex)
	}
	if strings.Contains(tab, telemetry.ShortTokens(2500)) {
		t.Errorf("the two token columns must never be added:\n%s", tab)
	}
	if !strings.Contains(tab, "no events: antigravity, cursor, kirocrew, opencode") {
		t.Errorf("harnesses without events are not listed:\n%s", tab)
	}
}

func TestReport_JSONCarriesTheTwoSections(t *testing.T) {
	whyFixtureEnv(t)
	var rep struct {
		Why struct {
			Decisions int `json:"decisions"`
			Harnesses []struct {
				Harness     string `json:"harness"`
				NotRecorded int    `json:"not_recorded"`
			} `json:"harnesses"`
		} `json:"why_spawns_changed"`
		Tokens []struct {
			Harness       string `json:"harness"`
			MovedObserved bool   `json:"moved_observed"`
			Moved         int64  `json:"tokens_moved_to_smaller_model"`
			Quota         struct {
				Status string `json:"status"`
			} `json:"quota_used"`
		} `json:"tokens_by_harness"`
	}
	if err := json.Unmarshal([]byte(runReportOut(t, "--days=0", "--json")), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Why.Decisions != 8 || len(rep.Why.Harnesses) != 2 || rep.Why.Harnesses[0].NotRecorded != 1 {
		t.Errorf("why = %+v", rep.Why)
	}
	got := map[string]bool{}
	for _, r := range rep.Tokens {
		got[r.Harness] = r.MovedObserved
		if r.Quota.Status != "unknown" {
			t.Errorf("%s quota = %q, want unknown with no snapshot", r.Harness, r.Quota.Status)
		}
		if r.Harness == "claude-code" && r.Moved != 1500 {
			t.Errorf("claude-code moved = %d, want 1500", r.Moved)
		}
	}
	if len(got) != 2 || !got["claude-code"] || got["codex"] {
		t.Errorf("observed by harness = %v, want claude-code observed and codex unobserved", got)
	}
}

// Owner decision O1: no money in these sections, in text or JSON.
func TestReport_WhyAndTokensSectionsHaveNoMoney(t *testing.T) {
	whyFixtureEnv(t)
	seedCompactorAndQuota(t)
	text := runReportOut(t, "--days=0")
	section := sectionOf(t, text, "WHY SPAWNS CHANGED")
	section = section[:strings.Index(section, "\nTokens saved (estimate)")]

	var rep map[string]json.RawMessage
	if err := json.Unmarshal([]byte(runReportOut(t, "--days=0", "--json")), &rep); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"why_spawns_changed", "tokens_by_harness"} {
		raw, ok := rep[key]
		if !ok {
			t.Fatalf("--json lacks %q", key)
		}
		section += "\n" + string(raw)
	}
	lower := strings.ToLower(section)
	for _, banned := range []string{"$", "cost", "price", "usd", "dollar"} {
		if strings.Contains(lower, banned) {
			t.Errorf("the routing note and tokens sections mention %q:\n%s", banned, section)
		}
	}
}

func TestReport_EmptyLogPrintsPlainSentences(t *testing.T) {
	endEnv(t)
	t.Setenv("DOWNSHIFT_QUOTA_FILE", filepath.Join(t.TempDir(), "quota.json"))
	text := runReportOut(t)
	for _, want := range []string{"No routing events in this window.", "No events in this window."} {
		if !strings.Contains(text, want) {
			t.Errorf("empty report lacks %q:\n%s", want, text)
		}
	}
	var rep struct {
		Why    accounting.RoutingWhy `json:"why_spawns_changed"`
		Tokens []json.RawMessage     `json:"tokens_by_harness"`
	}
	if err := json.Unmarshal([]byte(runReportOut(t, "--json")), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Why.Decisions != 0 || len(rep.Tokens) != 0 {
		t.Errorf("empty log gave %+v", rep)
	}
}
