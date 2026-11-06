// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"testing"
	"time"
)

func TestDetailFromReason(t *testing.T) {
	for _, tc := range []struct{ reason, want string }{
		{"terminal_escape_preserved", DetailANSI},
		{"carriage_return_preserved", DetailCR},
		{"no_final_newline_preserved", DetailNoFinalNewline},
		{"error_detected_fail_open_raw_preserved", DetailDangerWord},
		{"tsc_danger_word_preserved", DetailDangerWord},
		{"diff_secret_preserved", DetailDangerWord},
		{"turbo_danger_preserved", DetailDangerWord},
		{"maven_unrecognized_output_preserved", DetailUnknownShape},
		{"unrecognized_format", DetailUnknownShape},
		{"", DetailUnknownShape},
	} {
		if got := detailFromReason(tc.reason); got != tc.want {
			t.Errorf("detailFromReason(%q) = %q, want %q", tc.reason, got, tc.want)
		}
	}
}

// Each unrecognized_output run carries the detail of the exact point that
// decided it; every other passthrough reason carries none.
func TestCompactOutputDetail(t *testing.T) {
	for _, tc := range []struct {
		name       string
		data       string
		family     Family
		wantReason string
		wantDetail string
	}{
		{"danger word", "ok  \tpkg\t0.1s\npanic: boom\n", FamilyGoTest, PassthroughUnrecognized, DetailDangerWord},
		{"ansi escape", "\x1b[2Kprogress\nok  \tpkg\t0.1s\n", FamilyGoTest, PassthroughUnrecognized, DetailANSI},
		{"carriage return", "step 1\rstep 2\nok  \tpkg\t0.1s\n", FamilyGoTest, PassthroughUnrecognized, DetailCR},
		{"no final newline", "ok  \tpkg\t0.1s", FamilyGoTest, PassthroughUnrecognized, DetailNoFinalNewline},
		{"unknown shape", "hello there\nnothing known\n", FamilyGoTest, PassthroughUnrecognized, DetailUnknownShape},
		{"no compressor", "some output\n", noCompressorFamily(t), PassthroughUnrecognized, DetailNoCompressor},
		{"empty output has none", "", FamilyGoTest, PassthroughSmallOutput, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, reason, detail := compactOutput(0, []byte(tc.data), tc.family, false)
			if reason != tc.wantReason || detail != tc.wantDetail {
				t.Fatalf("got reason=%q detail=%q, want reason=%q detail=%q", reason, detail, tc.wantReason, tc.wantDetail)
			}
		})
	}
}

func TestCompactOutputNonzeroExitHasNoDetail(t *testing.T) {
	_, reason, detail := compactOutput(1, []byte("hello there\nnothing known\n"), FamilyGoTest, true)
	if reason != PassthroughNonZeroExit || detail != "" {
		t.Fatalf("got reason=%q detail=%q", reason, detail)
	}
}

func TestEventCarriesPassthroughDetail(t *testing.T) {
	ev := EventFrom(Result{Family: FamilyGoTest, Passthrough: PassthroughUnrecognized, PassthroughDetail: DetailCR}, time.Time{})
	if ev.PassthroughDetail != DetailCR {
		t.Fatalf("event detail = %q", ev.PassthroughDetail)
	}
	s := SummarizeEvents([]Event{ev, {Family: FamilyGoTest, Passthrough: PassthroughUnrecognized}})
	if s.PassthroughByDetail[DetailCR] != 1 || len(s.PassthroughByDetail) != 1 || s.PassthroughByReason[PassthroughUnrecognized] != 2 {
		t.Fatalf("summary: %+v", s)
	}
}

func TestDetailCodesAreClosedSet(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range PassthroughDetails {
		if d == "" || seen[d] {
			t.Fatalf("bad or duplicate detail %q", d)
		}
		seen[d] = true
	}
	for _, want := range []string{"danger_word", "ansi", "cr", "no_final_newline", "unknown_shape", "no_compressor"} {
		if !seen[want] {
			t.Errorf("missing detail %q", want)
		}
	}
}

// noCompressorFamily finds an allowlisted family without a compressor.
func noCompressorFamily(t *testing.T) Family {
	t.Helper()
	for _, argv := range [][]string{{"terraform", "plan"}, {"docker", "ps"}, {"kubectl", "get", "pods"}, {"make"}, {"aws", "s3", "ls"}} {
		if f := FamilyOf(argv); f != FamilyNone && !compactibleFamily(f) {
			return f
		}
	}
	t.Skip("every sampled family has a compressor")
	return FamilyNone
}
