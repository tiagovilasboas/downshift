// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package training

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

func e5Shadow(regex, nearest string, lift bool, lat float64, codes ...string) telemetry.Event {
	return telemetry.Event{Outcome: "allow", ReasonCodes: codes, E5Shadow: &telemetry.E5ShadowRecord{
		ModelID: "e5-prototypes-sha256:abc", RegexLabel: regex, NearestLabel: nearest, WouldLift: lift, LatencyMs: lat,
	}}
}

func e5ShadowErr(code string, lat float64) telemetry.Event {
	return telemetry.Event{Outcome: "allow", E5Shadow: &telemetry.E5ShadowRecord{
		ModelID: "e5-prototypes-sha256:abc", ErrorCode: code, LatencyMs: lat,
	}}
}

func e5Fuse(action string, lat float64) telemetry.Event {
	return telemetry.Event{Outcome: "rewrite_emitted", E5Fuse: &telemetry.E5FuseRecord{
		ModelID: "e5-prototypes-sha256:abc", NearestLabel: "MEDIUM", Action: action, LatencyMs: lat,
	}}
}

// TestSummarizeE5_CountsAndPercentiles fixes the arithmetic of every field on
// a mixed synthetic log: scored and failed observations, several regex
// labels, a reserved nearest label, two risk decisions (one the proxy counts,
// one it must not), and fuse actions with one fail-open.
func TestSummarizeE5_CountsAndPercentiles(t *testing.T) {
	events := []telemetry.Event{
		e5Shadow("TRIVIAL", "COMPLEX", true, 10, telemetry.ReasonClassifierConfident),
		e5Shadow("SIMPLE", "SIMPLE", false, 20),
		// Risk decision, no lift, nearest below regex: the proxy counts it.
		e5Shadow("MEDIUM", "SIMPLE", false, 30, telemetry.ReasonRiskCategory),
		// Risk decision whose nearest label is reserved: never ranks, not counted.
		e5Shadow("MEDIUM", "ABSTAIN", false, 40, telemetry.ReasonRiskFloor),
		e5Shadow("TRIVIAL", "TRIVIAL", false, 50),
		e5ShadowErr("timeout", 500),
		e5ShadowErr("missing_cmd", 0),
		e5ShadowErr("timeout", 500),
		// Cost-only and error records never count, even if they carried the object.
		{Outcome: telemetry.OutcomeUsage, E5Shadow: &telemetry.E5ShadowRecord{RegexLabel: "COMPLEX", NearestLabel: "TRIVIAL"}},
		{Outcome: "error", E5Fuse: &telemetry.E5FuseRecord{Action: "raise"}},
		e5Fuse("none", 5),
		e5Fuse("none", 15),
		e5Fuse("raise", 25),
		e5Fuse("abstain", 35),
		{Outcome: "allow", E5Fuse: &telemetry.E5FuseRecord{ErrorCode: "embed_error", LatencyMs: 1000}},
	}
	shadow, fuse := SummarizeE5(events)
	if shadow == nil || fuse == nil {
		t.Fatalf("shadow=%v fuse=%v", shadow, fuse)
	}
	wantShadow := &E5ShadowSummary{
		Observed:      8,
		Errors:        3,
		ErrorCodes:    map[string]int{"timeout": 2, "missing_cmd": 1},
		Scored:        5,
		WouldLift:     1,
		WouldLiftRate: 0.2,
		ByRegexLabel: []E5RegexLabelStats{
			{RegexLabel: "TRIVIAL", Scored: 2, WouldLift: 1, WouldLiftRate: 0.5},
			{RegexLabel: "SIMPLE", Scored: 1},
			{RegexLabel: "MEDIUM", Scored: 2},
		},
		NearestLabels:       map[string]int{"TRIVIAL": 1, "SIMPLE": 2, "COMPLEX": 1, "ABSTAIN": 1},
		Abstain:             1,
		AbstainShare:        0.2,
		RiskDecisions:       2,
		FalseDowngradeProxy: 1,
		// Sorted scored latencies 10 20 30 40 50: nearest-rank p50 is the
		// 3rd value, p95 the 5th.
		Latency: LatencyMs{Samples: 5, P50: 30, P95: 50, Max: 50},
	}
	if !reflect.DeepEqual(shadow, wantShadow) {
		t.Errorf("shadow summary\n got %+v\nwant %+v", shadow, wantShadow)
	}
	wantFuse := &E5FuseSummary{
		Observed:   5,
		Actions:    map[string]int{"none": 2, "raise": 1, "abstain": 1},
		Errors:     1,
		ErrorCodes: map[string]int{"embed_error": 1},
		// Answered latencies 5 15 25 35: p50 is the 2nd value, p95 the 4th.
		Latency: LatencyMs{Samples: 4, P50: 15, P95: 35, Max: 35},
	}
	if !reflect.DeepEqual(fuse, wantFuse) {
		t.Errorf("fuse summary\n got %+v\nwant %+v", fuse, wantFuse)
	}

	report := SummarizeShadow(nil)
	report.E5Shadow, report.E5Fuse = shadow, fuse
	var text bytes.Buffer
	report.WriteText(&text)
	for _, want := range []string{"\nE5 shadow\n", "\nE5 fuse\n", "regex TRIVIAL", "false downgrade proxy      1", "missing_cmd 1, timeout 2", "none 2, abstain 1, raise 1", "p50 30  p95 50  max 50"} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("text output missing %q:\n%s", want, text.String())
		}
	}
	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(raw, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"e5_shadow", "e5_fuse"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("JSON report missing key %q: %s", k, raw)
		}
	}
	if strings.Contains(strings.ToLower(string(raw)), "prompt") {
		t.Errorf("report must stay prompt-free: %s", raw)
	}
}

// TestNearestRankPercentile pins the percentile definition on small samples.
func TestNearestRankPercentile(t *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		want    LatencyMs
	}{
		{"empty", nil, LatencyMs{}},
		{"one", []float64{7}, LatencyMs{Samples: 1, P50: 7, P95: 7, Max: 7}},
		{"two", []float64{9, 3}, LatencyMs{Samples: 2, P50: 3, P95: 9, Max: 9}},
		{"twenty", seq(20), LatencyMs{Samples: 20, P50: 10, P95: 19, Max: 20}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := latencyStats(tc.samples); got != tc.want {
				t.Errorf("got %+v want %+v", got, tc.want)
			}
		})
	}
}

func seq(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64(i + 1)
	}
	return out
}

// TestSummarizeE5_NoneLeavesReportUnchanged: a log without E5 objects adds
// no key to the JSON and no E5 heading to the text, so pre-E5 output is
// byte-identical.
func TestSummarizeE5_NoneLeavesReportUnchanged(t *testing.T) {
	plain := []telemetry.Event{{Outcome: "allow", Complexity: "MEDIUM"}, {Outcome: telemetry.OutcomeUsage}}
	shadow, fuse := SummarizeE5(plain)
	if shadow != nil || fuse != nil {
		t.Fatalf("expected nil summaries, got %+v %+v", shadow, fuse)
	}
	before, _ := json.Marshal(SummarizeShadow(nil))
	report := SummarizeShadow(nil)
	report.E5Shadow, report.E5Fuse = shadow, fuse
	after, _ := json.Marshal(report)
	if !bytes.Equal(before, after) {
		t.Errorf("JSON changed without E5 events:\n%s\n%s", before, after)
	}
	if strings.Contains(string(after), "e5_") {
		t.Errorf("JSON must not carry E5 keys without E5 events: %s", after)
	}
	var text bytes.Buffer
	report.WriteText(&text)
	if strings.Contains(text.String(), "E5 shadow") || strings.Contains(text.String(), "E5 fuse") {
		t.Errorf("text must not print an E5 section without E5 events:\n%s", text.String())
	}
	if !strings.Contains(text.String(), "Candidate classifier") {
		t.Errorf("text should still print the candidate section:\n%s", text.String())
	}
}
