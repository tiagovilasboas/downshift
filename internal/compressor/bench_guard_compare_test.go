// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"os"
	"strings"
	"testing"
)

func TestCompareBaselineAggregates(t *testing.T) {
	base := []baseRow{{rowTotal, 3, 1400, 500}}
	if m := compareBaseline(base, []baseRow{{rowTotal, 3, 1400, 504}}); len(m) != 0 {
		t.Fatalf("0.3 point drop is inside the tolerance: %v", m)
	}
	m := compareBaseline(base, []baseRow{{rowTotal, 3, 1400, 540}})
	if len(m) != 1 || !strings.Contains(m[0], "@total: saving fell") {
		t.Fatalf("want a total drop, got %v", m)
	}
}

func TestCompareBaseline(t *testing.T) {
	base := []baseRow{{"a", 2, 1000, 400}, {"b", 1, 400, 100}}
	cases := []struct {
		name string
		got  []baseRow
		want string // substring of the single expected message; "" means none
	}{
		{"equal", base, ""},
		{"improved", []baseRow{{"a", 2, 1000, 300}, {"b", 1, 400, 100}}, ""},
		{"drop beyond tolerance", []baseRow{{"a", 2, 1000, 430}, {"b", 1, 400, 100}}, "a: saving fell from 60.0% to 57.0%"},
		{"tolerance edge", []baseRow{{"a", 2, 1000, 420}, {"b", 1, 400, 100}}, ""},
		{"byte floor", []baseRow{{"a", 2, 1000, 400}, {"b", 1, 400, 108}}, ""},
		{"fewer fixtures", []baseRow{{"a", 1, 1000, 400}, {"b", 1, 400, 100}}, "a: 1 fixtures, baseline has 2"},
		{"vanished family", []baseRow{{"a", 2, 1000, 400}}, "b: family vanished"},
		{"new family", append([]baseRow{{"c", 1, 100, 50}}, base...), "c: compacts"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msgs := compareBaseline(base, c.got)
			if c.want == "" {
				if len(msgs) != 0 {
					t.Fatalf("unexpected: %v", msgs)
				}
				return
			}
			if len(msgs) != 1 || !strings.Contains(msgs[0], c.want) || !strings.Contains(msgs[0], "make bench-baseline") {
				t.Fatalf("want one message with %q and the regen command, got %v", c.want, msgs)
			}
		})
	}
}

// intendedCR lists the fixtures whose carriage returns are the point: CRLF
// transcripts and terminal progress redraws. Any other fixture holding a CR was
// mangled by a checkout or an editor.
var intendedCR = map[string]bool{
	"dart_pass_ansi.txt": true, "dart_pass_compact.txt": true, "flutter_fail.txt": true,
	"flutter_pass_compact.txt": true, "flutter_print_output.txt": true,
	"git_push_progress_preserve.txt": true, "gitstatus_clean_crlf.txt": true,
	"gotest_crlf.txt": true, "progress_clean.txt": true, "progress_danger.txt": true,
	"progress_shrinking.txt": true,
}

func TestCorpusNoUnintendedCR(t *testing.T) {
	for _, name := range allFixtures(t) {
		in, err := os.ReadFile(corpusDir + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if has := strings.Contains(string(in), "\r"); has != intendedCR[name] {
			t.Errorf("%s: contains CR = %v, intended = %v (see .gitattributes and intendedCR)", name, has, intendedCR[name])
		}
	}
}
