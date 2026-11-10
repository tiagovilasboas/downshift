// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"fmt"
	"strings"
	"testing"
)

func TestBalanceLogKeepsHeadTailAndFailures(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 2000; i++ {
		switch i {
		case 700:
			b.WriteString("--- FAIL: TestBroken (0.01s)\n")
		case 701:
			b.WriteString("    broken_test.go:12: want 1, got 2\n")
		default:
			fmt.Fprintf(&b, "=== RUN   TestCase%d/error_path\n", i)
		}
	}
	out, ok := Balance(BalanceLog, []byte(b.String()))
	if !ok {
		t.Fatal("a 2000-line log was not cut")
	}
	s := string(out)
	for _, want := range []string{"TestCase0/", "--- FAIL: TestBroken", "want 1, got 2", "TestCase1999/", "lines cut (downshift retrieve)"} {
		if !strings.Contains(s, want) {
			t.Errorf("cut log lacks %q", want)
		}
	}
	if strings.Contains(s, "TestCase1000/") {
		t.Error("a scaffolding line whose test name holds a failure word was kept")
	}
}

func TestBalanceDiffKeepsEveryHeader(t *testing.T) {
	var b strings.Builder
	for f := 0; f < 5; f++ {
		fmt.Fprintf(&b, "diff --git a/f%d.go b/f%d.go\n--- a/f%d.go\n+++ b/f%d.go\n@@ -1,300 +1,300 @@\n", f, f, f, f)
		for i := 0; i < 300; i++ {
			fmt.Fprintf(&b, "+line %d of file %d\n", i, f)
		}
	}
	out, ok := Balance(BalanceDiff, []byte(b.String()))
	if !ok {
		t.Fatal("a 1500-line diff was not cut")
	}
	s := string(out)
	for f := 0; f < 5; f++ {
		if !strings.Contains(s, fmt.Sprintf("diff --git a/f%d.go", f)) {
			t.Errorf("file %d header lost", f)
		}
	}
	if strings.Count(s, "@@ -1,300 +1,300 @@") != 5 || !strings.Contains(s, "more lines in this hunk") {
		t.Errorf("hunk headers or the cut note missing:\n%s", s[:200])
	}
}

func TestBalanceMatchesCapsPerFileAndCounts(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "a.go:%d:match %s\n", i, strings.Repeat("x", 400))
	}
	out, ok := Balance(BalanceMatches, []byte(b.String()))
	if !ok {
		t.Fatal("500 matches were not cut")
	}
	s := string(out)
	if strings.Count(s, "a.go:") != balanceMatchPerFile+1 || !strings.Contains(s, "480 more matches in 1 files") || !strings.Contains(s, "more chars") {
		t.Errorf("matches not capped per file, counted or line-cut:\n%s", s[len(s)-300:])
	}
}

func TestBalanceSmallGainKeepsText(t *testing.T) {
	text := []byte(strings.Repeat("plain line\n", 150))
	if out, ok := Balance(BalanceLog, text); ok || string(out) != string(text) {
		t.Error("a log just over head+tail was cut for a small gain")
	}
}
