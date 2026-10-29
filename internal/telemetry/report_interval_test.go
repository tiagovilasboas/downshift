// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestPrintReport_Interval(t *testing.T) {
	cases := []struct {
		name       string
		routed     func(i int) float64
		wantZero   bool
		wantSubstr string
	}{
		{"interval shown, excludes zero", func(i int) float64 { return 0.05 + 0.01*float64(i%5) }, false, "95% interval of tokens saved per task (control minus routed): +"},
		{"includes zero says not established", func(i int) float64 { return 0.10 + 0.05*float64(i%4) }, true, "not established: the interval includes zero"},
	}
	control := func(i int) float64 { return 0.10 + 0.05*float64(i%4) }
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture()
			for i := 0; i < 30; i++ {
				f.spawn(GroupRouted, tc.routed(i), ChildCompleted, "s1")
				f.spawn(GroupControl, control(i)+map[bool]float64{true: 0, false: 0.1}[tc.wantZero], ChildCompleted, "s1")
			}
			rep := BuildReport(f.events, 30)
			var b bytes.Buffer
			PrintReport(&b, rep)
			out := b.String()
			if strings.Contains(out, "not an interval") {
				t.Errorf("old wording still printed:\n%s", out)
			}
			if !strings.Contains(out, tc.wantSubstr) {
				t.Errorf("want %q in:\n%s", tc.wantSubstr, out)
			}
			if got := strings.Contains(out, "includes zero"); got != tc.wantZero {
				t.Errorf("includes-zero sentence = %v, want %v:\n%s", got, tc.wantZero, out)
			}
			raw, _ := ReportJSON(rep)
			var top map[string]json.RawMessage
			if err := json.Unmarshal([]byte(raw), &top); err != nil {
				t.Fatal(err)
			}
			if _, ok := top["tokens_interval"]; !ok {
				t.Errorf("--json must carry tokens_interval: %s", raw)
			}
		})
	}
}

func TestReportJSON_NoIntervalBelowMinimum(t *testing.T) {
	raw, _ := ReportJSON(BuildReport(nil, 30))
	if strings.Contains(raw, "tokens_interval") {
		t.Errorf("no interval without the 20-task minimum: %s", raw)
	}
}

func TestPrintSegmentedReport_HeaderShowsWindow(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{Timestamp: now.Add(-time.Hour).Format(time.RFC3339Nano), Harness: "codex", Outcome: OutcomeAllow, CorrelationID: "a"},
		{Timestamp: now.Add(-time.Minute).Format(time.RFC3339Nano), Harness: "cursor", Outcome: OutcomeAllow, CorrelationID: "b"},
	}
	for _, tc := range []struct {
		days int
		want string
	}{{7, "Routed vs control, last 7 days"}, {0, "Routed vs control, all time"}} {
		var b bytes.Buffer
		PrintSegmentedReport(&b, events, tc.days)
		if !strings.Contains(b.String(), tc.want) {
			t.Errorf("days=%d: want %q in:\n%s", tc.days, tc.want, b.String())
		}
	}
}
