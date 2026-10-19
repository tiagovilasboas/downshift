// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Placeholder model ids: the token view never prices anything itself, it reads
// the costs the usage record already carries.
const (
	modelBig   = "model-big"
	modelSmall = "model-small"
)

type tokUse struct {
	in, out, cached, write int64
}

func tokDecision(id, outcome, verdict, from, to string, mods ...func(*telemetry.Event)) telemetry.Event {
	ev := telemetry.Event{
		CorrelationID: id, Harness: "claude-code", SessionID: "s1", Outcome: outcome,
		Verdict: verdict, FromModel: from, ToModel: to,
	}
	for _, m := range mods {
		m(&ev)
	}
	return ev
}

// tokUsage builds a priced usage record linked to a decision. baseline < 0
// leaves the baseline cost absent (an unknown requested model).
func tokUsage(link, harness, from, ran string, u tokUse, actual, baseline float64) telemetry.Event {
	ev := telemetry.Event{
		CorrelationID: "u-" + link + "-" + harness, Harness: harness, SessionID: "s1", Outcome: telemetry.OutcomeUsage,
		LinkedDecision: link, FromModel: from, ToModel: ran, ActualCostUSD: &actual,
		InputTokens: &u.in, OutputTokens: &u.out, CachedTokens: &u.cached, CacheWriteTokens: &u.write,
	}
	if baseline >= 0 {
		ev.BaselineCostUSD = &baseline
		ev.BaselineSource = telemetry.BaselineRequested
	}
	return ev
}

func bucketOf(t *testing.T, ht telemetry.HarnessTokens, dir string) telemetry.TokenClasses {
	t.Helper()
	var c *telemetry.TokenClasses
	switch dir {
	case telemetry.TokenCheaper:
		c = ht.Cheaper
	case telemetry.TokenPricier:
		c = ht.Pricier
	case telemetry.TokenUnchanged:
		c = ht.Unchanged
	case telemetry.TokenUnattributed:
		c = ht.Unattributed
	}
	if c == nil {
		t.Fatalf("bucket %s is nil for an observed harness", dir)
	}
	return *c
}

func TestTokensRerouted_Placement(t *testing.T) {
	use := tokUse{in: 100, out: 200, cached: 1000, write: 300}
	const total = 100 + 200 + 1000
	applied := func(verdict, from, to string) telemetry.Event {
		return tokDecision("d", telemetry.OutcomeRewriteEmitted, verdict, from, to)
	}
	cases := []struct {
		name   string
		events []telemetry.Event
		want   string // bucket that receives the record
		pair   string // "requested>ran" expected in by_pair, "" for none
	}{
		{"applied downshift is cheaper", []telemetry.Event{applied("DOWNSHIFT", modelBig, modelSmall),
			tokUsage("d", "claude-code", modelBig, modelSmall, use, 1, 4)}, telemetry.TokenCheaper, modelBig + ">" + modelSmall},
		{"applied upshift goes to pricier", []telemetry.Event{applied("UPSHIFT", modelSmall, modelBig),
			tokUsage("d", "claude-code", modelSmall, modelBig, use, 4, 1)}, telemetry.TokenPricier, modelSmall + ">" + modelBig},
		{"allow is unchanged", []telemetry.Event{tokDecision("d", telemetry.OutcomeAllow, "OK", modelBig, modelBig),
			tokUsage("d", "claude-code", modelBig, modelBig, use, 4, 4)}, telemetry.TokenUnchanged, modelBig + ">" + modelBig},
		{"held downshift (guardrail) is unchanged", []telemetry.Event{
			tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall, func(e *telemetry.Event) { e.Corrections = []string{"R1"} }),
			tokUsage("d", "claude-code", modelBig, modelBig, use, 4, 4)}, telemetry.TokenUnchanged, modelBig + ">" + modelBig},
		{"refuted rewrite is unchanged and ran on the requested model", []telemetry.Event{
			applied("DOWNSHIFT", modelBig, modelSmall),
			{CorrelationID: "r", Harness: "claude-code", Outcome: telemetry.OutcomeResolved, LinkedDecision: "d", RewriteHonored: boolPtr(false)},
			tokUsage("d", "claude-code", modelBig, modelSmall, use, 1, 4)}, telemetry.TokenUnchanged, modelBig + ">" + modelBig},
		{"control group is unchanged", []telemetry.Event{
			tokDecision("d", telemetry.OutcomeBaseline, "DOWNSHIFT", modelBig, modelBig, func(e *telemetry.Event) { e.ReasonCodes = []string{telemetry.ReasonControl} }),
			tokUsage("d", "claude-code", modelBig, modelBig, use, 4, 4)}, telemetry.TokenUnchanged, modelBig + ">" + modelBig},
		{"no-route baseline is unchanged", []telemetry.Event{
			tokDecision("d", telemetry.OutcomeBaseline, "DOWNSHIFT", modelBig, modelBig),
			tokUsage("d", "claude-code", modelBig, modelBig, use, 4, 4)}, telemetry.TokenUnchanged, modelBig + ">" + modelBig},
		{"unlinked usage is unattributed", []telemetry.Event{
			tokUsage("nope", "claude-code", modelBig, modelSmall, use, 1, 4)}, telemetry.TokenUnattributed, ""},
		{"applied rewrite with unknown baseline is unattributed", []telemetry.Event{applied("DOWNSHIFT", "unknown", modelSmall),
			tokUsage("d", "claude-code", "unknown", modelSmall, use, 1, -1)}, telemetry.TokenUnattributed, ""},
		{"applied rewrite priced the same is unchanged", []telemetry.Event{applied("DOWNSHIFT", modelBig, modelSmall),
			tokUsage("d", "claude-code", modelBig, modelSmall, use, 2, 2)}, telemetry.TokenUnchanged, modelBig + ">" + modelSmall},
		{"usage with no decision order still links (usage first)", []telemetry.Event{
			tokUsage("d", "claude-code", modelBig, modelSmall, use, 1, 4), applied("DOWNSHIFT", modelBig, modelSmall)},
			telemetry.TokenCheaper, modelBig + ">" + modelSmall},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tr := telemetry.Aggregate(tc.events).TokensRerouted
			ht := tr.ByHarness["claude-code"]
			if ht.Status != telemetry.TokenStatusObserved {
				t.Fatalf("status = %q, want observed", ht.Status)
			}
			for _, dir := range []string{telemetry.TokenCheaper, telemetry.TokenPricier, telemetry.TokenUnchanged, telemetry.TokenUnattributed} {
				c := bucketOf(t, ht, dir)
				if dir == tc.want {
					if c.Records != 1 || c.Input != 100 || c.Output != 200 || c.CacheRead != 700 || c.CacheWrite != 300 || c.Total != total {
						t.Errorf("%s = %+v, want 1 record with in 100 / out 200 / cache read 700 / cache write 300", dir, c)
					}
				} else if c.Total != 0 || c.Records != 0 {
					t.Errorf("%s = %+v, want empty", dir, c)
				}
			}
			var gotPair string
			for _, p := range ht.ByPair {
				gotPair = p.Requested + ">" + p.Ran
				if p.Direction != tc.want {
					t.Errorf("pair direction = %s, want %s", p.Direction, tc.want)
				}
			}
			if gotPair != tc.pair {
				t.Errorf("pair = %q, want %q", gotPair, tc.pair)
			}
			if len(ht.ByModel) != 1 || ht.ByModel[0].Total != total {
				t.Errorf("token mix = %+v, want one model with %d tokens", ht.ByModel, total)
			}
			if tr.Totals == nil {
				t.Fatalf("totals must exist when a harness is observed")
			}
			if got := tr.Totals.Cheaper.Total + tr.Totals.Pricier.Total + tr.Totals.Unchanged.Total + tr.Totals.Unattributed.Total; got != total {
				t.Errorf("totals partition = %d, want %d", got, total)
			}
		})
	}
}

// The mix counts the model that ran, so a refuted rewrite is on the requested model.
func TestTokensRerouted_MixFollowsTheModelThatRan(t *testing.T) {
	use := tokUse{in: 10, out: 20}
	events := []telemetry.Event{
		tokDecision("a", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall),
		tokUsage("a", "claude-code", modelBig, modelSmall, use, 1, 4),
		tokDecision("b", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall),
		{CorrelationID: "r", Harness: "claude-code", Outcome: telemetry.OutcomeResolved, LinkedDecision: "b", RewriteHonored: boolPtr(false)},
		tokUsage("b", "claude-code", modelBig, modelSmall, use, 4, 4),
	}
	ht := telemetry.Aggregate(events).TokensRerouted.ByHarness["claude-code"]
	got := map[string]int64{}
	for _, m := range ht.ByModel {
		got[m.Model] = m.Total
	}
	if got[modelSmall] != 30 || got[modelBig] != 30 || len(got) != 2 {
		t.Errorf("token mix = %v, want 30 on each model", got)
	}
}

func TestTokensRerouted_CacheSplit(t *testing.T) {
	cases := []struct {
		name        string
		u           tokUse
		read, write int64
	}{
		{"read and write", tokUse{cached: 1000, write: 300}, 700, 300},
		{"split unknown: all cached is read", tokUse{cached: 500}, 500, 0},
		{"write above cached is clamped", tokUse{in: 1, cached: 100, write: 900}, 0, 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := []telemetry.Event{
				tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall),
				tokUsage("d", "claude-code", modelBig, modelSmall, tc.u, 1, 4),
			}
			c := bucketOf(t, telemetry.Aggregate(ev).TokensRerouted.ByHarness["claude-code"], telemetry.TokenCheaper)
			if c.CacheRead != tc.read || c.CacheWrite != tc.write || c.Cache() != tc.read+tc.write {
				t.Errorf("cache read/write = %d/%d, want %d/%d", c.CacheRead, c.CacheWrite, tc.read, tc.write)
			}
		})
	}
}

// A record with no price or no tokens is not a priced token observation.
func TestTokensRerouted_SkipsUnpricedAndEmpty(t *testing.T) {
	unpriced := tokUsage("d", "claude-code", modelBig, modelSmall, tokUse{in: 5}, 1, 4)
	unpriced.ActualCostUSD = nil
	empty := tokUsage("d", "claude-code", modelBig, modelSmall, tokUse{}, 1, 4)
	ev := []telemetry.Event{
		tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall), unpriced, empty,
	}
	tr := telemetry.Aggregate(ev).TokensRerouted
	if ht := tr.ByHarness["claude-code"]; ht.Status != telemetry.TokenStatusNoneInRange || ht.Cheaper != nil {
		t.Errorf("claude-code = %+v, want none_in_window with no buckets", ht)
	}
	if tr.Totals != nil {
		t.Errorf("totals = %+v, want nil", tr.Totals)
	}
}

// A harness with no token source reads unobserved, from the capability table,
// and carries no bucket that could read as a measured zero.
func TestTokensRerouted_UnobservedHarnessFromCapabilities(t *testing.T) {
	tr := telemetry.Aggregate(nil).TokensRerouted
	for _, n := range core.AllHarnessCaps() {
		ht, ok := tr.ByHarness[n.Harness]
		if !ok {
			t.Errorf("%s missing from by_harness", n.Harness)
			continue
		}
		wantSource := n.Caps.UsageSource
		if wantSource == "" {
			wantSource = core.UsageNone
		}
		wantStatus := telemetry.TokenStatusNoneInRange
		if wantSource == core.UsageNone {
			wantStatus = telemetry.TokenStatusUnobserved
		}
		if ht.UsageSource != wantSource || ht.Status != wantStatus {
			t.Errorf("%s = source %q status %q, want %q / %q", n.Harness, ht.UsageSource, ht.Status, wantSource, wantStatus)
		}
		if ht.Cheaper != nil || ht.Pricier != nil || ht.Unchanged != nil || ht.Unattributed != nil {
			t.Errorf("%s: an unobserved harness must carry no bucket", n.Harness)
		}
	}
	for _, h := range []string{"cursor", "codex", "opencode"} {
		if tr.ByHarness[h].Status != telemetry.TokenStatusUnobserved {
			t.Errorf("%s status = %q, want unobserved", h, tr.ByHarness[h].Status)
		}
	}
	raw, err := json.Marshal(tr.ByHarness["cursor"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "cheaper") || strings.Contains(string(raw), `"total"`) {
		t.Errorf("unobserved harness JSON shows a count: %s", raw)
	}
}

// Observed harnesses are kept apart, and a harness outside the capability
// table with records still appears.
func TestTokensRerouted_PerHarness(t *testing.T) {
	use := tokUse{in: 10, out: 10}
	ev := []telemetry.Event{
		tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall),
		tokUsage("d", "claude-code", modelBig, modelSmall, use, 1, 4),
		tokDecision("e", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall, func(e *telemetry.Event) { e.Harness = "custom-harness" }),
		tokUsage("e", "custom-harness", modelBig, modelSmall, tokUse{in: 5}, 1, 4),
	}
	tr := telemetry.Aggregate(ev).TokensRerouted
	if got := bucketOf(t, tr.ByHarness["claude-code"], telemetry.TokenCheaper).Total; got != 20 {
		t.Errorf("claude-code cheaper = %d, want 20", got)
	}
	custom := tr.ByHarness["custom-harness"]
	if custom.Status != telemetry.TokenStatusObserved || custom.UsageSource != "unknown" || bucketOf(t, custom, telemetry.TokenCheaper).Total != 5 {
		t.Errorf("custom-harness = %+v", custom)
	}
	if tr.Totals.Cheaper.Total != 25 || tr.Totals.Cheaper.Records != 2 {
		t.Errorf("totals cheaper = %+v, want 25 tokens in 2 records", tr.Totals.Cheaper)
	}
	order := tr.HarnessOrder()
	if order[0] != "claude-code" || order[len(order)-1] != "custom-harness" {
		t.Errorf("harness order = %v, want declared harnesses first and unknown ones last", order)
	}
}

func TestExportSummary_TokensRerouted(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	stamp := now.Format(time.RFC3339Nano)
	d := tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall)
	d.Timestamp = stamp
	u := tokUsage("d", "claude-code", modelBig, modelSmall, tokUse{in: 40, out: 90, cached: 1000, write: 100}, 1, 4)
	u.Timestamp = stamp
	out, err := telemetry.ExportJSON(telemetry.ExportSummary([]telemetry.Event{d, u}, 7, now))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Tokens struct {
			Basis     string `json:"basis"`
			ByHarness map[string]struct {
				Source  string `json:"usage_source"`
				Status  string `json:"status"`
				Cheaper *struct {
					Records    int   `json:"records"`
					Input      int64 `json:"input"`
					Output     int64 `json:"output"`
					CacheRead  int64 `json:"cache_read"`
					CacheWrite int64 `json:"cache_write"`
					Total      int64 `json:"total"`
				} `json:"cheaper"`
				Pricier *json.RawMessage `json:"pricier"`
				ByPair  []struct {
					Requested string `json:"requested_model"`
					Ran       string `json:"ran_model"`
					Direction string `json:"direction"`
					Total     int64  `json:"total"`
				} `json:"by_pair"`
				ByModel []struct {
					Model string `json:"model"`
					Total int64  `json:"total"`
				} `json:"by_model"`
			} `json:"by_harness"`
			Totals *struct {
				Cheaper struct {
					Total int64 `json:"total"`
				} `json:"cheaper"`
			} `json:"totals"`
		} `json:"tokens_rerouted"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("export is not valid JSON: %v\n%s", err, out)
	}
	cc := doc.Tokens.ByHarness["claude-code"]
	if cc.Source != "transcript" || cc.Status != "observed" || cc.Cheaper == nil || cc.Pricier == nil {
		t.Fatalf("claude-code = %+v", cc)
	}
	if c := *cc.Cheaper; c.Records != 1 || c.Input != 40 || c.Output != 90 || c.CacheRead != 900 || c.CacheWrite != 100 || c.Total != 1130 {
		t.Errorf("cheaper = %+v", c)
	}
	if len(cc.ByPair) != 1 || cc.ByPair[0].Requested != modelBig || cc.ByPair[0].Ran != modelSmall || cc.ByPair[0].Direction != "cheaper" {
		t.Errorf("by_pair = %+v", cc.ByPair)
	}
	if len(cc.ByModel) != 1 || cc.ByModel[0].Model != modelSmall || cc.ByModel[0].Total != 1130 {
		t.Errorf("by_model = %+v", cc.ByModel)
	}
	if doc.Tokens.Totals == nil || doc.Tokens.Totals.Cheaper.Total != 1130 {
		t.Errorf("totals = %+v", doc.Tokens.Totals)
	}
	cursor := doc.Tokens.ByHarness["cursor"]
	if cursor.Status != "unobserved" || cursor.Source != "none" || cursor.Cheaper != nil {
		t.Errorf("cursor = %+v, want unobserved with no counts", cursor)
	}
	if doc.Tokens.Basis == "" {
		t.Error("the basis label must travel with the numbers")
	}
	for _, banned := range []string{"session_id", "agent_hash", "correlation", "linked_decision"} {
		if strings.Contains(out, banned) {
			t.Errorf("export carries %q", banned)
		}
	}
}

func TestPrintStats_TokensLine(t *testing.T) {
	tests := []struct {
		name   string
		events []telemetry.Event
		want   string
	}{
		{"cheaper tokens with a breakdown", []telemetry.Event{
			tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall),
			tokUsage("d", "claude-code", modelBig, modelSmall, tokUse{in: 40_000, out: 90_000, cached: 1_100_000, write: 100_000}, 1, 4)},
			"Tokens moved to a cheaper model: 1.2M (in 40k / out 90k / cache 1.1M) · to a pricier model: 0"},
		{"pricier tokens", []telemetry.Event{
			tokDecision("d", telemetry.OutcomeRewriteEmitted, "UPSHIFT", modelSmall, modelBig),
			tokUsage("d", "claude-code", modelSmall, modelBig, tokUse{in: 10, out: 20}, 4, 1)},
			"Tokens moved to a cheaper model: 0 · to a pricier model: 30 (in 10 / out 20 / cache 0)"},
		{"no priced usage reads unobserved, not zero", []telemetry.Event{
			tokDecision("d", telemetry.OutcomeAllow, "OK", modelBig, modelBig)},
			"Tokens moved to a cheaper model: unobserved (no priced child usage in this window)"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var b bytes.Buffer
			telemetry.PrintStats(tc.events, telemetry.StatsOptions{}, &b)
			lines := strings.Split(b.String(), "\n")
			if lines[3] != tc.want {
				t.Errorf("line 4 = %q, want %q\n%s", lines[3], tc.want, b.String())
			}
			if !strings.Contains(b.String(), "Estimated at list price, not an invoice") {
				t.Errorf("the dollar basis label must stay:\n%s", b.String())
			}
			if strings.Contains(b.String(), "tokens saved") {
				t.Errorf("Downshift does not save tokens:\n%s", b.String())
			}
		})
	}
}

func TestPrintStats_TokensDetail(t *testing.T) {
	ev := []telemetry.Event{
		tokDecision("d", telemetry.OutcomeRewriteEmitted, "DOWNSHIFT", modelBig, modelSmall),
		tokUsage("d", "claude-code", modelBig, modelSmall, tokUse{in: 10, out: 20, cached: 100, write: 40}, 1, 4),
	}
	var b bytes.Buffer
	telemetry.PrintStats(ev, telemetry.StatsOptions{Detail: true}, &b)
	text := b.String()
	for _, want := range []string{
		"Tokens rerouted, by harness",
		"cheaper", "model-big -> model-small",
		"cache read 60, write 40",
		"token mix by model: model-small 130",
		"cursor       unobserved (usage source: none)",
		"codex        unobserved",
		"Estimated at list price, not an invoice",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("--detail is missing %q:\n%s", want, text)
		}
	}
}
