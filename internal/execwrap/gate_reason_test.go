// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// syncBuf is one writer for both streams, safe for the HoldBound timer.
type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

// G3: a run the gate released records why: too_large, stderr_output or
// hold_timeout. A streamed run with no gate reason and a non-zero exit stays
// nonzero_exit. Every byte is written, in write order, either way.
func TestGate_RecordsWhyItStreamed(t *testing.T) {
	big := bytes.Repeat([]byte("line of output\n"), compressor.MaxProcessableBytes/15+10)
	cases := []struct {
		name  string
		setup func(g *gate)
		write func(g *gate)
		exit  int
		want  string
	}{
		{"over the hold limit without keep", nil, func(g *gate) { _, _ = stdoutWriter{g}.Write(big) }, 0, PassthroughTooLarge},
		{"stderr over the hold limit", nil, func(g *gate) {
			_, _ = stdoutWriter{g}.Write([]byte("a\n"))
			_, _ = stderrWriter{g}.Write(big)
		}, 0, PassthroughTooLarge},
		{"too large wins over a failed exit", nil, func(g *gate) { _, _ = stdoutWriter{g}.Write(big) }, 1, PassthroughTooLarge},
		{"stderr with a terminal on stdin", func(g *gate) { g.stderrReleases = true }, func(g *gate) {
			_, _ = stdoutWriter{g}.Write([]byte("a\n"))
			_, _ = stderrWriter{g}.Write([]byte("Password: "))
		}, 0, PassthroughStderr},
		{"hold bound", func(*gate) {
			old := HoldBound
			HoldBound = 10 * time.Millisecond
			t.Cleanup(func() { HoldBound = old })
		}, func(g *gate) {
			_, _ = stdoutWriter{g}.Write([]byte("a\n"))
			for end := time.Now().Add(5 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
				g.mu.Lock()
				done := g.streaming
				g.mu.Unlock()
				if done {
					break
				}
			}
		}, 0, PassthroughHoldTimeout},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var merged syncBuf
			g := newGate(&merged, &merged, true, []string{"make"}, FamilyMake)
			if tc.setup != nil {
				tc.setup(g)
			}
			tc.write(g)
			raw, emitted, compacted, _, reason := g.finish(tc.exit, FamilyMake)
			if compacted || reason != tc.want {
				t.Fatalf("compacted=%v reason=%q, want %q", compacted, reason, tc.want)
			}
			if raw != emitted || int64(merged.b.Len()) != raw {
				t.Fatalf("raw=%d emitted=%d printed=%d", raw, emitted, merged.b.Len())
			}
		})
	}
	g := newGate(io.Discard, io.Discard, false, []string{"make"}, FamilyMake)
	if _, _, _, _, reason := g.finish(2, FamilyMake); reason != PassthroughNonZeroExit {
		t.Errorf("a run that never held and failed: reason %q, want %q", reason, PassthroughNonZeroExit)
	}
}

// The gate replays held output in the order it was written, each byte on its
// own stream, for a family that merges stderr into its report and one that
// does not; a compacted run prints held stderr whole after stdout.
func TestGate_ReplaysInWriteOrder(t *testing.T) {
	writes := []struct {
		isErr bool
		s     string
	}{{false, "o1\n"}, {true, "e1\n"}, {true, "e2\n"}, {false, "o2\n"}, {true, "e3\n"}}
	for _, f := range []Family{FamilyMake, FamilyGoTest} {
		var merged syncBuf
		var out, errb bytes.Buffer
		for _, w := range []struct{ o, e io.Writer }{{&merged, &merged}, {&out, &errb}} {
			g := newGate(w.o, w.e, true, []string{"x"}, f)
			for _, wr := range writes {
				if wr.isErr {
					_, _ = stderrWriter{g}.Write([]byte(wr.s))
				} else {
					_, _ = stdoutWriter{g}.Write([]byte(wr.s))
				}
			}
			if len(g.order) != 4 {
				t.Errorf("%s: %d segments, want 4 (adjacent writes coalesce)", f, len(g.order))
			}
			g.finish(1, f)
		}
		if got := merged.b.String(); got != "o1\ne1\ne2\no2\ne3\n" {
			t.Errorf("%s: merged replay %q", f, got)
		}
		if out.String() != "o1\no2\n" || errb.String() != "e1\ne2\ne3\n" {
			t.Errorf("%s: stdout %q stderr %q", f, out.String(), errb.String())
		}
	}
	var merged syncBuf
	g := newGate(&merged, &merged, true, []string{"go", "test"}, FamilyMake)
	_, _ = stderrWriter{g}.Write([]byte("warning: x\n"))
	_, _ = stdoutWriter{g}.Write([]byte(passingSuiteText()))
	if _, _, compacted, _, _ := g.finish(0, FamilyMake); !compacted || !strings.HasSuffix(merged.b.String(), "]\nwarning: x\n") {
		t.Errorf("compacted=%v output %q", compacted, merged.b.String())
	}
}

// passingSuiteText is a clean go test run the make family folds.
func passingSuiteText() string {
	var b strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		b.WriteString("ok  \tgithub.com/x/internal/" + p + "\t0.1s\n")
	}
	return b.String()
}

// Only a terminal counts as interactive: a reader in memory, a pipe, a file
// and the null device do not, so a stderr byte from them is held.
func TestInteractiveStdin(t *testing.T) {
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer pr.Close()
	defer pw.Close()
	file, err := os.Create(filepath.Join(t.TempDir(), "in.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var nilFile *os.File
	for name, r := range map[string]io.Reader{
		"reader": strings.NewReader("x"), "null device": null, "pipe": pr, "file": file, "nil file": nilFile, "nil": nil,
	} {
		if interactiveStdin(r) {
			t.Errorf("%s counted as a terminal", name)
		}
	}
}
