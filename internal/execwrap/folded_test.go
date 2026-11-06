// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

// goTestOK is a passing go test report the lossless compressor shortens.
func goTestOK() []byte {
	var b strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&b, "=== RUN   TestCase%d\n--- PASS: TestCase%d (0.00s)\n", i, i)
	}
	b.WriteString("PASS\nok  \texample.com/pkg\t0.120s\n")
	return []byte(b.String())
}

func TestGateFinish_FoldedFlag(t *testing.T) {
	keepOK := func([]byte) (string, bool) { return "0123456789ab", true }
	cases := []struct {
		name          string
		raw           []byte
		keep          func([]byte) (string, bool)
		wantCompacted bool
		wantFolded    bool
	}{
		{"lossless compact", goTestOK(), nil, true, false},
		{"lossless wins over keep", goTestOK(), keepOK, true, false},
		{"fold path", repeatRun(), keepOK, true, true},
		{"fold refused", repeatRun(), func([]byte) (string, bool) { return "", false }, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			g := newGate(&out, &errb, true, []string{"go", "test"}, FamilyGoTest)
			g.keep = tc.keep
			g.buf.Write(tc.raw)
			g.nOut = int64(len(tc.raw))
			_, _, compacted, folded, _ := g.finish(0, FamilyGoTest)
			if compacted != tc.wantCompacted || folded != tc.wantFolded {
				t.Fatalf("compacted=%v folded=%v, want %v %v", compacted, folded, tc.wantCompacted, tc.wantFolded)
			}
		})
	}
}

func TestEventFrom_Folded(t *testing.T) {
	now := time.Unix(0, 0)
	if ev := EventFrom(Result{Family: FamilyGoTest, Compacted: true, Folded: true}, now); !ev.Folded || !ev.Compacted {
		t.Fatalf("ev = %+v", ev)
	}
	if ev := EventFrom(Result{Family: FamilyGoTest, Compacted: true}, now); ev.Folded {
		t.Fatalf("ev = %+v", ev)
	}
}

func TestSummarizeEvents_SplitsFolded(t *testing.T) {
	evs := []Event{
		{Family: FamilyGoTest, RawBytes: 1000, EmittedBytes: 100, Compacted: true},
		{Family: FamilyGoTest, RawBytes: 800, EmittedBytes: 300, Compacted: true, Folded: true},
		{Family: FamilyGoTest, RawBytes: 50, EmittedBytes: 50, Passthrough: PassthroughSmallOutput},
	}
	s := SummarizeEvents(evs)
	if s.Commands != 3 || s.Compacted != 2 || s.SavedBytes != 1400 {
		t.Fatalf("totals changed: %+v", s)
	}
	if s.FoldedRuns != 1 || s.FoldedSavedBytes != 500 {
		t.Fatalf("folded = %d runs, %d bytes", s.FoldedRuns, s.FoldedSavedBytes)
	}
	if s.EstimatedTokensSaved != 350 {
		t.Fatalf("tokens = %d", s.EstimatedTokensSaved)
	}
}
