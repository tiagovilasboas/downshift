// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package benchmark_test

import (
	"bytes"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/benchmark"
	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// The context benchmark prints measured compressor bytes for the checked-in
// fixtures. Routing cost and accuracy stay not measured. 68/35/80 are not
// evidence.
func TestCompareContextScenarios_PrintsCompressorBytes(t *testing.T) {
	var buf bytes.Buffer
	if err := benchmark.CompareContextScenarios(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "not measured") {
		t.Fatalf("routing cost and accuracy must stay not measured:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "bytes") {
		t.Fatalf("byte unit missing:\n%s", out)
	}
	for _, banned := range []string{"68%", "35%", "80%", "68 %", "35 %", "80 %"} {
		if strings.Contains(out, banned) {
			t.Fatalf("output cites %s as evidence:\n%s", banned, out)
		}
	}
	if regexp.MustCompile(`(?i)\$\s*\d`).MatchString(out) {
		t.Fatalf("output cites dollar savings:\n%s", out)
	}
	if regexp.MustCompile(`(?i)token(?:s)? (?:count|saved|reduction)\s*[:=]?\s*\d`).MatchString(out) {
		t.Fatalf("output cites a token count:\n%s", out)
	}

	fixtures := []struct {
		labels []string
		file   string
		saves  bool
	}{
		{[]string{"go_test_ok", "go test all-ok"}, "fixtures/go_test_all_ok.txt", true},
		{[]string{"go_test_warning", "go test non-ok warning"}, "fixtures/go_test_non_ok.txt", false},
		{[]string{"repetitive_logs", "repetitive logs"}, "fixtures/repetitive_logs.txt", true},
		{[]string{"search_hits", "search hits"}, "fixtures/search_hits.txt", false},
	}
	for _, fx := range fixtures {
		raw, err := os.ReadFile(fx.file)
		if err != nil {
			t.Fatalf("checked-in fixture %s: %v", fx.file, err)
		}
		res := compressor.CompressExit(raw, compressor.ModeSafe, 0)
		savings := res.OriginalBytes - res.ReducedBytes
		if savings < 0 {
			savings = 0
		}
		if fx.saves && savings <= 0 {
			t.Fatalf("%s fixture did not reduce bytes (%d -> %d)", fx.file, res.OriginalBytes, res.ReducedBytes)
		}
		if !fx.saves && (savings != 0 || res.ReducedBytes != res.OriginalBytes) {
			t.Fatalf("%s fixture changed bytes (%d -> %d)", fx.file, res.OriginalBytes, res.ReducedBytes)
		}
		got, ok := fixtureBytes(out, fx.labels)
		if !ok {
			t.Fatalf("missing byte row %q in:\n%s", fx.labels[0], out)
		}
		if got[0] != res.OriginalBytes || got[1] != res.ReducedBytes || got[2] != savings {
			t.Fatalf("%s printed %v, compressor measured original=%d reduced=%d savings=%d",
				fx.labels[0], got, res.OriginalBytes, res.ReducedBytes, savings)
		}
	}
}

var (
	lineRow = regexp.MustCompile(`(?m)^(\S+)\s+original=(\d+) bytes reduced=(\d+) bytes savings=(\d+) bytes\s*$`)
	pipeRow = regexp.MustCompile(`(?m)^(.{1,40}?)\s+\|\s+(\d+)\s+\|\s+(\d+)\s+\|\s+(\d+)\s*$`)
)

func fixtureBytes(out string, labels []string) ([3]int, bool) {
	want := map[string]bool{}
	for _, label := range labels {
		want[label] = true
	}
	for _, match := range lineRow.FindAllStringSubmatch(out, -1) {
		if want[match[1]] {
			return triple(match[2], match[3], match[4]), true
		}
	}
	for _, match := range pipeRow.FindAllStringSubmatch(out, -1) {
		if want[strings.TrimSpace(match[1])] {
			return triple(match[2], match[3], match[4]), true
		}
	}
	return [3]int{}, false
}

func triple(a, b, c string) [3]int {
	original, _ := strconv.Atoi(a)
	reduced, _ := strconv.Atoi(b)
	savings, _ := strconv.Atoi(c)
	return [3]int{original, reduced, savings}
}
