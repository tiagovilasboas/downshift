// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// bigGrep is a fake git that prints n match lines (about 34 bytes each), on
// stdout, then exits with code.
func bigGrep(t *testing.T, n int, extra string) {
	t.Helper()
	fakeBin(t, "git", "yes 'src/a.go:12:some match text here' | head -n "+itoa(n)+"; "+extra)
}

// storeKeep is a keep port that remembers what it stored.
type storeKeep struct {
	got   []byte
	calls int
	ok    bool
}

func (s *storeKeep) keep(raw []byte) (string, bool) {
	s.calls++
	s.got = append([]byte(nil), raw...)
	return "0123456789ab", s.ok
}

// G1: output over the hold limit is spooled when CCR keeps every run and
// balanced compaction is on. The run prints its head, the bytes cut with the
// retrieve id, its tail and the marker; the keep port gets every byte; the
// exit code is the program's own.
func TestRun_OverHoldLimitIsSpooledAndCut(t *testing.T) {
	const lines = 100000 // ~3.4 MB, over compressor.MaxProcessableBytes
	cases := []struct {
		name     string
		extra    string
		wantCode int
	}{
		{"clean run", "", 0},
		{"failed run keeps its exit code", "exit 3", 3},
		{"stderr passes through while spooling", "echo 'grep: warning' >&2", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bigGrep(t, lines, tc.extra)
			store := &storeKeep{ok: true}
			var out, errb bytes.Buffer
			r := Run([]string{"git", "grep", "match"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb,
				Compact: true, Keep: store.keep, Balanced: true})
			if r.ExitCode != tc.wantCode {
				t.Fatalf("exit = %d, want %d", r.ExitCode, tc.wantCode)
			}
			raw := int64(lines * len("src/a.go:12:some match text here\n"))
			if int64(len(store.got)) != raw || store.calls != 1 {
				t.Fatalf("kept %d bytes in %d calls, want %d in 1", len(store.got), store.calls, raw)
			}
			s := out.String()
			if out.Len() >= 64<<10 || !r.Compacted || !r.Folded || r.KeptID != "0123456789ab" || r.Passthrough != "" {
				t.Fatalf("printed %d bytes, result %+v", out.Len(), r)
			}
			for _, want := range []string{"src/a.go:12:some match text here\n", "bytes cut (downshift retrieve 0123456789ab)", "full output: downshift retrieve 0123456789ab]"} {
				if !strings.Contains(s, want) {
					t.Errorf("output lacks %q:\n%.300s", want, s)
				}
			}
			if strings.Contains(tc.extra, ">&2") && errb.String() != "grep: warning\n" {
				t.Errorf("stderr = %q", errb.String())
			}
			if r.EmittedBytes != int64(out.Len()+errb.Len()) || r.RawBytes != raw+int64(errb.Len()) {
				t.Errorf("raw=%d emitted=%d, printed %d+%d", r.RawBytes, r.EmittedBytes, out.Len(), errb.Len())
			}
		})
	}
}

// Over the hold limit with no way to keep the raw output, the run streams raw
// as before, and the reason is recorded: too_large.
func TestRun_OverHoldLimitWithoutKeepIsTooLarge(t *testing.T) {
	const lines = 70000
	raw := lines * len("src/a.go:12:some match text here\n")
	failing := &storeKeep{ok: false}
	cases := []struct {
		name string
		o    Options
		code int
	}{
		{"no keep port", Options{Balanced: true}, 0},
		{"balanced off", Options{Keep: (&storeKeep{ok: true}).keep}, 0},
		{"keep failed only", Options{KeepFailed: (&storeKeep{ok: true}).keep, Balanced: true}, 0},
		{"store refuses", Options{Keep: failing.keep, Balanced: true}, 0},
		{"failed run without keep", Options{Balanced: true}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra := ""
			if tc.code != 0 {
				extra = "exit " + itoa(tc.code)
			}
			bigGrep(t, lines, extra)
			var out, errb bytes.Buffer
			o := tc.o
			o.Stdin, o.Stdout, o.Stderr, o.Compact = strings.NewReader(""), &out, &errb, true
			r := Run([]string{"git", "grep", "match"}, o)
			if out.Len() != raw || r.Compacted || r.Passthrough != PassthroughTooLarge || r.ExitCode != tc.code {
				t.Fatalf("printed %d of %d bytes, result %+v", out.Len(), raw, r)
			}
		})
	}
}

// A spool past SpoolLimit is written out raw and the rest streams.
func TestRun_SpoolPastLimitStreamsRaw(t *testing.T) {
	old := SpoolLimit
	SpoolLimit = 3 << 20
	t.Cleanup(func() { SpoolLimit = old })
	const lines = 120000 // ~4 MB
	bigGrep(t, lines, "")
	store := &storeKeep{ok: true}
	var out, errb bytes.Buffer
	r := Run([]string{"git", "grep", "match"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb,
		Compact: true, Keep: store.keep, Balanced: true})
	want := strings.Repeat("src/a.go:12:some match text here\n", lines)
	if out.String() != want || r.Compacted || r.Passthrough != PassthroughTooLarge || store.calls != 0 {
		t.Fatalf("printed %d of %d bytes, result %+v, keep calls %d", out.Len(), len(want), r, store.calls)
	}
}

// Memory stays bounded while spooling: nothing is held in the gate's buffers
// and the spool keeps only its head and a bounded tail.
func TestGate_SpoolBoundsMemory(t *testing.T) {
	var out, errb bytes.Buffer
	g := newGate(&out, &errb, true, []string{"git", "grep", "x"}, FamilyGitGrep)
	store := &storeKeep{ok: true}
	g.keep, g.balanced = store.keep, true
	chunk := []byte(strings.Repeat("src/a.go:1:x\n", 5000)) // 65 KB
	w := stdoutWriter{g}
	total := 0
	for total <= 3*compressor.MaxProcessableBytes {
		_, _ = w.Write(chunk)
		total += len(chunk)
		if g.spool != nil && (g.heldLen() != 0 || len(g.spool.head) > spoolHeadBytes || cap(g.spool.tail) > 4*spoolTailBytes) {
			t.Fatalf("held=%d head=%d tail cap=%d", g.heldLen(), len(g.spool.head), cap(g.spool.tail))
		}
	}
	if g.spool == nil {
		t.Fatal("output over the hold limit was not spooled")
	}
	name := g.spool.f.Name()
	_, emitted, compacted, _, _ := g.finish(0, FamilyGitGrep)
	if !compacted || len(store.got) != total || emitted >= 64<<10 || out.Len() != int(emitted) {
		t.Fatalf("compacted=%v kept=%d/%d emitted=%d", compacted, len(store.got), total, emitted)
	}
	if _, err := os.Lstat(name); err == nil {
		t.Errorf("spool file %s was not removed", name)
	}
}
