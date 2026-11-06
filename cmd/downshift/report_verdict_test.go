// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// writeVerdictLog writes a ledger where every "class" spawn failed and was
// redone a minute later in its session. It also holds two baseline lines an
// older binary wrote (its control group, with the retired CONTROL code, and
// its no-route switch) that the report must read and leave out. It returns
// the correlation ids of the failed class decisions.
func writeVerdictLog(t *testing.T, n int) []string {
	t.Helper()
	path := os.Getenv("DOWNSHIFT_EVENT_LOG")
	at := time.Now().UTC().Add(-time.Hour)
	var failed []string
	put := func(ev telemetry.Event) {
		if err := telemetry.AppendTo(path, ev); err != nil {
			t.Fatal(err)
		}
	}
	spawn := func(i int, session string, status telemetry.ChildStatus) string {
		at = at.Add(time.Minute)
		id := fmt.Sprintf("%016x%016x", at.UnixNano(), i)
		put(telemetry.Event{
			CorrelationID: id, Timestamp: at.Format(time.RFC3339Nano), Harness: "claude-code",
			SessionID: session, Outcome: telemetry.OutcomeRewriteEmitted,
			RoutingNote: &telemetry.RoutingNote{Verdict: "class"},
		})
		tokens := int64(1000)
		put(telemetry.Event{
			CorrelationID: "u" + id[1:], Timestamp: at.Add(10 * time.Second).Format(time.RFC3339Nano),
			Harness: "claude-code", SessionID: session, Outcome: telemetry.OutcomeUsage, LinkedDecision: id, InputTokens: &tokens,
		})
		put(telemetry.Event{
			CorrelationID: "e" + id[1:], Timestamp: at.Add(20 * time.Second).Format(time.RFC3339Nano),
			Harness: "claude-code", SessionID: session, Outcome: telemetry.OutcomeChildEnded, LinkedDecision: id, ChildStatus: status,
		})
		return id
	}
	for i := 0; i < n; i++ {
		sess := fmt.Sprintf("class-%d", i)
		failed = append(failed, spawn(i, sess, telemetry.ChildError))
		spawn(i, sess, telemetry.ChildCompleted)
	}
	old := func(id, codes string) string {
		return `{"correlation_id":"` + id + `","timestamp":"` + time.Now().UTC().Add(-30*time.Minute).Format(time.RFC3339Nano) +
			`","harness":"claude-code","session_id":"old","outcome":"baseline","verdict":"DOWNSHIFT"` + codes + `}` + "\n"
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, old("old-control", `,"reason_codes":["CONTROL"]`)...)
	raw = append(raw, old("old-noroute", ``)...)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return failed
}

// The text report is routed-only: one routed row, then the rework per verdict
// with the policy key that turns each verdict off, and the excluded line for
// the two old baseline lines. --json carries no control field and lists the
// decisions that were followed by rework.
func TestReport_ReworkByVerdict(t *testing.T) {
	endEnv(t)
	failed := writeVerdictLog(t, 20)

	var out, errOut bytes.Buffer
	if rc := runReport([]string{"--days=30", "--text"}, &out, &errOut); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errOut.String())
	}
	text := out.String()
	for _, want := range []string{
		"Routed spawns, last 30 days",
		"Rework by routing verdict, routed spawns\n",
		"routing.by_class: false",
		"2 spawn(s) left unrouted by a session pause, or logged as baseline by an older version",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text report lacks %q:\n%s", want, text)
		}
	}
	for _, gone := range []string{"control", "Control", "worse", "not enough data", "DOWNSHIFT_NO_ROUTE", "DOWNSHIFT_CONTROL_SHARE"} {
		if strings.Contains(text, gone) {
			t.Errorf("text report must not mention %q:\n%s", gone, text)
		}
	}

	out.Reset()
	if rc := runReport([]string{"--days=30", "--json"}, &out, &errOut); rc != 0 {
		t.Fatalf("rc=%d %s", rc, errOut.String())
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &top); err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"control", "control_share_observed", "min_tasks_per_group", "enough_data", "message", "tokens_interval"} {
		if _, ok := top[gone]; ok {
			t.Errorf("report --json must not carry %q", gone)
		}
	}
	var excluded int
	if err := json.Unmarshal(top["excluded_unrouted"], &excluded); err != nil || excluded != 2 {
		t.Errorf("excluded_unrouted = %s, want 2 (%v)", top["excluded_unrouted"], err)
	}
	var rep struct {
		Routed struct {
			Spawns int `json:"spawns"`
		} `json:"routed"`
		ByVerdict []struct {
			Verdict string   `json:"verdict"`
			TurnOff string   `json:"turn_off"`
			IDs     []string `json:"reworked_correlation_ids"`
		} `json:"rework_by_verdict"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Routed.Spawns != 40 {
		t.Errorf("routed spawns = %d, want 40: the old baseline lines are not routed", rep.Routed.Spawns)
	}
	if len(rep.ByVerdict) != 1 || rep.ByVerdict[0].Verdict != "class" || rep.ByVerdict[0].TurnOff != "routing.by_class: false" {
		t.Fatalf("rework_by_verdict = %+v", rep.ByVerdict)
	}
	if got, want := strings.Join(rep.ByVerdict[0].IDs, ","), strings.Join(failed, ","); got != want {
		t.Errorf("reworked ids = %s, want %s", got, want)
	}
}
