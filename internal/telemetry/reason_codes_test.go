// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

var traceCat = catalog.Load()

// routeTrace routes prompt through core.Route (the real decision path, no
// optional layers) and returns the event the shared runner would record.
func routeTrace(t *testing.T, prompt string, current core.Tier, known bool) telemetry.Event {
	t.Helper()
	t.Setenv(core.NBUpshiftEnv, "")
	t.Setenv(core.NBDownshiftEnv, "")
	t.Setenv("DOWNSHIFT_GRAPHIFY_CMD", "")
	currentID := ""
	if known {
		currentID = traceCat.ModelFor("claude-code", current).ID
	}
	d := core.Route(prompt, "claude-code", currentID, traceCat)
	return fromcore.FromDecision(d, "0123456789abcdef", "test")
}

// trace is the event the shared runner records for a decision.
func trace(d core.Decision) telemetry.Event {
	return fromcore.FromDecision(d, "0123456789abcdef", "test")
}

// Decisions with the facts a routing decision carries; the reason codes read
// these facts, whichever policy produced them.
var (
	heldRiskDownshift = core.Decision{
		Harness: "claude-code", Tier: core.TierFrontier, Risk: []string{"authz"},
		Guards:      core.Guards{RiskFloor: true},
		Corrections: []string{core.RuleRiskDowngrade, core.RuleUnconfidentDowngrade},
	}
	heldUnknownMid = core.Decision{
		Harness: "claude-code", Tier: core.TierMid, NoSignal: true,
		Corrections: []string{core.RuleUnconfidentUnknownMid},
	}
	heldLanguage = core.Decision{
		Harness: "claude-code", Tier: core.TierFrontier, UnsupportedLanguage: true,
		Corrections: []string{core.RuleUnconfidentDowngrade},
	}
	confidentDownshift = core.Decision{Harness: "claude-code", Tier: core.TierSmall, Confident: true}
	noSignalOnMid      = core.Decision{Harness: "claude-code", Tier: core.TierMid, NoSignal: true}
)

func TestReasonCodes_FromDecisions(t *testing.T) {
	cases := []struct {
		name string
		d    core.Decision
		want []string
	}{
		{"risk floor held downshift", heldRiskDownshift,
			[]string{"CLASSIFIER_UNCONFIDENT", "HELD_RISK_DOWNSHIFT", "HELD_UNCONFIDENT_DOWNSHIFT", "RISK_CATEGORY", "RISK_FLOOR"}},
		{"R9 hold on unknown current model", heldUnknownMid,
			[]string{"CLASSIFIER_UNCONFIDENT", "HELD_UNCONFIDENT_UNKNOWN_MID", "NO_SIGNAL"}},
		{"unsupported language held", heldLanguage,
			[]string{"CLASSIFIER_UNCONFIDENT", "HELD_UNCONFIDENT_DOWNSHIFT", "UNSUPPORTED_LANGUAGE"}},
		{"plain confident downshift", confidentDownshift, []string{"CLASSIFIER_CONFIDENT"}},
		{"no signal, current model already mid", noSignalOnMid, []string{"CLASSIFIER_UNCONFIDENT", "NO_SIGNAL"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ev := trace(tc.d)
			if ev.RoutingPolicy != telemetry.RoutingPolicy {
				t.Errorf("routing_policy = %q, want %q", ev.RoutingPolicy, telemetry.RoutingPolicy)
			}
			if !reflect.DeepEqual(ev.ReasonCodes, tc.want) {
				t.Errorf("reason_codes = %v, want %v (verdict %s, corrections %v)", ev.ReasonCodes, tc.want, ev.Verdict, ev.Corrections)
			}
			if !slices.IsSorted(ev.ReasonCodes) {
				t.Errorf("reason_codes not sorted: %v", ev.ReasonCodes)
			}
			if ev.Confidence != nil {
				t.Error("confidence must stay absent: no calibrated probability exists")
			}
		})
	}
}

func TestUnsupportedLanguage_VisibleInStatsAndWhy(t *testing.T) {
	ev := trace(heldLanguage)
	ev.Outcome = telemetry.OutcomeAllow
	ev.SessionID = "language-session"
	var stats bytes.Buffer
	telemetry.PrintStats([]telemetry.Event{ev}, telemetry.StatsOptions{Detail: true}, &stats)
	if !strings.Contains(stats.String(), "Language guard        1 of 1") || !strings.Contains(stats.String(), telemetry.ReasonUnsupportedLanguage) {
		t.Fatalf("language reason missing from stats: %s", stats.String())
	}
	why := telemetry.Why([]telemetry.Event{ev}, ev.SessionID)
	if !strings.Contains(why, telemetry.ReasonUnsupportedLanguage) || !strings.Contains(why, "English and Portuguese rule coverage") {
		t.Fatalf("language reason missing from why: %s", why)
	}
	if got := telemetry.Aggregate([]telemetry.Event{ev}).ReasonCodes[telemetry.ReasonUnsupportedLanguage]; got != 1 {
		t.Fatalf("language count = %d, want 1", got)
	}
}

// Every hold maps to its own code, and the codes follow Corrections exactly.
func TestReasonCodes_EveryRuleHasACode(t *testing.T) {
	rules := []string{
		core.RuleUnconfidentDowngrade, core.RuleComplexDowngrade, core.RuleForeignModel,
		core.RuleRiskDowngrade, core.RuleUnconfidentUnknownSmall, core.RuleUnconfidentUnknownFrontier,
		core.RuleUnconfidentUpshift, core.RuleUnconfidentUnknownMid,
	}
	for _, r := range rules {
		codes := fromcore.ReasonCodes(core.Decision{Confident: true, Corrections: []string{r}})
		if len(codes) != 2 || !strings.HasPrefix(codes[1], "HELD_") || codes[1] == telemetry.ReasonHeldUnmapped {
			t.Errorf("rule %s -> %v, want CLASSIFIER_CONFIDENT plus one specific HELD_ code", r, codes)
		}
	}
	codes := fromcore.ReasonCodes(core.Decision{Confident: true, Corrections: []string{"R99_FUTURE"}})
	if !slices.Contains(codes, telemetry.ReasonHeldUnmapped) {
		t.Errorf("unknown rule must still leave a hold code, got %v", codes)
	}
	lifts := fromcore.ReasonCodes(core.Decision{
		Session: core.Session{
			CreditHeld: true,
		},
		Guards: core.Guards{
			RiskFloor:   true,
			PolicyFloor: true,
		},
		Shadow: core.Shadow{
			NBUpshift:   core.NBUpshift{Would: true, Applied: true},
			NBDownshift: core.NBDownshift{Would: true, Applied: false},
		},
	})
	want := []string{"CLASSIFIER_UNCONFIDENT", "CREDIT_HELD", "NB_UPSHIFT_APPLIED", "RISK_FLOOR", "TIER_FLOOR"}
	if !reflect.DeepEqual(lifts, want) {
		t.Errorf("lift codes = %v, want %v (a shadow-only NB opinion is not a reason)", lifts, want)
	}
}

var codeShape = regexp.MustCompile(`^[A-Z][A-Z_]*$`)

// No code may carry prompt text, a matched word, a risk category name or a path.
func TestReasonCodes_NeverContainPromptText(t *testing.T) {
	prompts := []string{
		"rotate the zqxsecretmarker API key in /etc/zqxpathmarker/config.yaml and drop the users table",
		"refactor auth middleware zqxauthmarker to verify JWT signatures",
		"look at the zqxlogsmarker folder",
	}
	for _, p := range prompts {
		for _, known := range []bool{true, false} {
			ev := routeTrace(t, p, core.TierFrontier, known)
			for _, c := range ev.ReasonCodes {
				if !codeShape.MatchString(c) {
					t.Errorf("code %q is not an upper-case constant", c)
				}
			}
			raw, err := json.Marshal(ev)
			if err != nil {
				t.Fatal(err)
			}
			lower := strings.ToLower(string(raw))
			for _, marker := range []string{"zqx", "/etc", "config.yaml", "users table", "middleware"} {
				if strings.Contains(lower, marker) {
					t.Errorf("event JSON leaks %q: %s", marker, raw)
				}
			}
			for _, cat := range core.RiskCategories(p) {
				for _, c := range ev.ReasonCodes {
					if strings.Contains(strings.ToLower(c), strings.ToLower(cat)) {
						t.Errorf("code %q names risk category %q", c, cat)
					}
				}
			}
		}
	}
}

func TestReasonCodes_JSONRoundTrip(t *testing.T) {
	ev := routeTrace(t, "rename the token validation function and make it accept unsigned JWTs", core.TierFrontier, true)
	raw, err := json.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"routing_policy":"deterministic-v1"`) || !strings.Contains(string(raw), `"reason_codes":[`) {
		t.Fatalf("trace keys missing: %s", raw)
	}
	if strings.Contains(string(raw), `"confidence"`) {
		t.Fatalf("top-level confidence must be absent: %s", raw)
	}
	var back telemetry.Event
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, ev) {
		t.Fatalf("round trip changed the event:\n got %+v\nwant %+v", back, ev)
	}
}

// A line written before the trace existed parses unchanged and counts as
// before; it adds no reason codes to stats or the export.
func TestReasonCodes_OldEventsParseUnchanged(t *testing.T) {
	old := `{"correlation_id":"0123456789abcdef","timestamp":"2026-10-01T10:00:00Z","source":"hook","agent":"subagent","harness":"cursor","complexity":"SIMPLE","requested_model":"a","final_model":"b","requested_reasoning_effort":"unknown","final_reasoning_effort":"low","verdict":"DOWNSHIFT","tier":"SMALL","policy_version":"core-route-v1","binary_version":"0.9.0","outcome":"rewrite_emitted","estimated_savings":0.5,"safe_verdict":"OK","corrections":["R1_UNCONFIDENT_DOWNSHIFT"]}`
	path := filepath.Join(t.TempDir(), "events.jsonl")
	if err := os.WriteFile(path, []byte(old+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	events, err := telemetry.ReadEventsFrom(path)
	if err != nil || len(events) != 1 {
		t.Fatalf("parse old line: %v, %d events", err, len(events))
	}
	ev := events[0]
	if ev.RoutingPolicy != "" || ev.ReasonCodes != nil || ev.Confidence != nil {
		t.Fatalf("old event gained trace fields: %+v", ev)
	}
	if ev.Verdict != "DOWNSHIFT" || ev.PolicyVersion != "core-route-v1" || !slices.Equal(ev.Corrections, []string{"R1_UNCONFIDENT_DOWNSHIFT"}) {
		t.Fatalf("old fields changed: %+v", ev)
	}
	again, _ := json.Marshal(ev)
	for _, key := range []string{"routing_policy", "reason_codes", `"confidence"`} {
		if strings.Contains(string(again), key) {
			t.Errorf("re-encoded old event gained %s: %s", key, again)
		}
	}
	s := telemetry.Aggregate(events)
	if s.Total != 1 || s.Corrected != 1 || len(s.ReasonCodes) != 0 {
		t.Fatalf("old event stats changed: total=%d corrected=%d codes=%v", s.Total, s.Corrected, s.ReasonCodes)
	}
	out, _ := telemetry.ExportJSON(telemetry.ExportSummary(events, 0, time.Now()))
	if strings.Contains(out, "reason_codes") {
		t.Errorf("export of old events must not carry reason_codes:\n%s", out)
	}
}

func TestReasonCodes_CountedInExport(t *testing.T) {
	a := trace(confidentDownshift)
	b := trace(heldUnknownMid)
	exp := telemetry.ExportSummary([]telemetry.Event{a, b}, 0, time.Now())
	if exp.ReasonCodes["CLASSIFIER_CONFIDENT"] != 1 || exp.ReasonCodes["HELD_UNCONFIDENT_UNKNOWN_MID"] != 1 || exp.ReasonCodes["NO_SIGNAL"] != 1 {
		t.Fatalf("export reason_codes = %v", exp.ReasonCodes)
	}
}
