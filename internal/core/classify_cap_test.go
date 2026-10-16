// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestClassifyHeadRuneBoundary(t *testing.T) {
	// "é" is two bytes; an odd prefix length puts the 32 KB cut inside one.
	p := "a" + strings.Repeat("é", maxClassifyBytes)
	h := classifyHead(p)
	if !utf8.ValidString(h) {
		t.Fatal("head cut inside a rune")
	}
	if len(h) > maxClassifyBytes || len(h) < maxClassifyBytes-utf8.UTFMax {
		t.Fatalf("head length %d", len(h))
	}
	short := strings.Repeat("x", maxClassifyBytes)
	if classifyHead(short) != short {
		t.Fatal("prompt at the cap must be untouched")
	}
}

func TestClassifySignalPastCapIgnored(t *testing.T) {
	pad := func(n int) string { return strings.Repeat("lorem ipsum dolor ", n/18+1)[:n] }

	// A strong COMPLEX signal at byte 100 is found.
	early := pad(100) + " redesign the distributed architecture and migrate the database " + pad(60000)
	if got := Classify(early); got.Complexity != Complex {
		t.Errorf("signal at byte ~100: got %v, want COMPLEX (scores %v)", got.Complexity, got.Scores)
	}

	// The same signal at byte 40 000 is ignored by design.
	late := pad(40000) + " redesign the distributed architecture and migrate the database " + pad(1000)
	got := Classify(late)
	if got.Complexity == Complex || !got.NoSignal {
		t.Errorf("signal at byte 40000: got %v NoSignal=%v, want ignored (scores %v)", got.Complexity, got.NoSignal, got.Scores)
	}
}

// Word-count rules keep using the full text: a long prompt whose only signal
// sits in the head still gets the 80-word COMPLEX nudge.
func TestClassifyWordCountUsesFullText(t *testing.T) {
	head := "rename the variable foo"
	short := Classify(head)
	long := Classify(head + " " + strings.Repeat("word ", 20000))
	if long.Scores[Complex] != short.Scores[Complex]+1 {
		t.Errorf("long prompt COMPLEX score %d, short %d; want +1 from the word-count rule",
			long.Scores[Complex], short.Scores[Complex])
	}
}

func TestClassify900KBUnder500ms(t *testing.T) {
	skipUnderRace(t)
	filler := "refactor the payment module and update the tests for the token handler. "
	prompt := strings.Repeat(filler, (900<<10)/len(filler)+1)[:900<<10]
	// Warm regex/risk caches; the first call on a shared CI runner can be 2–3× slower.
	_ = Classify(prompt)
	start := time.Now()
	_ = Classify(prompt)
	elapsed := time.Since(start)
	t.Logf("Classify 900KB (warm): %v", elapsed)
	// 1 s warm path: order-of-magnitude guard. Full wall-clock including semantic
	// fuse is in TestClassifyLatencyLargePrompts (900 KB, 10 s).
	if elapsed > time.Second {
		t.Errorf("Classify of 900 KB took %v, warm limit 1s", elapsed)
	}
}
