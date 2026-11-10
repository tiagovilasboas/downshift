// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// G2: stderr of a family outside stderrIsReport is held with stdout. Output
// that stays raw is replayed with each byte on its own stream. (The order of
// writes across the two pipes is fixed by the gate, see
// TestGate_ReplaysInWriteOrder; at run level the OS may read either pipe
// first.)
func TestRun_StderrHeldStaysOnItsStream(t *testing.T) {
	fakeBin(t, "make", "echo out1; echo err1 >&2; echo out2; echo err2 >&2; exit 2")
	r, out, errOut := run(t, []string{"make", "x"}, true)
	if out != "out1\nout2\n" || errOut != "err1\nerr2\n" || r.EmittedBytes != r.RawBytes || r.ExitCode != 2 {
		t.Fatalf("stdout=%q stderr=%q result %+v", out, errOut, r)
	}
	if r.Compacted || r.Passthrough != PassthroughNonZeroExit {
		t.Fatalf("result %+v", r)
	}
}

// makeLog writes a fake make whose stdout is n plain lines with failure lines
// at the given line numbers, plus one warning on stderr, and exits with code.
func makeLog(t *testing.T, n int, failAt map[int]string, code int) int {
	t.Helper()
	var b strings.Builder
	for i := 0; i < n; i++ {
		if f, ok := failAt[i]; ok {
			b.WriteString(f + "\n")
			continue
		}
		b.WriteString(buildLine(i) + "\n")
	}
	src := filepath.Join(t.TempDir(), "out.txt")
	if err := os.WriteFile(src, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	fakeBin(t, "make", "head -n "+itoa(n/2)+" "+src+"; echo 'warning: deprecated flag' >&2; tail -n +"+itoa(n/2+1)+" "+src+"; exit "+itoa(code))
	return b.Len()
}

// A large make run with one stderr warning is cut once CCR keeps its raw
// stdout (a clean one may already be cut by the generic chain), and the
// warning is printed whole on stderr. A failed run keeps
// every failure line and its exit code.
func TestRun_LargeMakeWithStderrIsBalanced(t *testing.T) {
	cases := []struct {
		name   string
		failAt map[int]string
		code   int
		want   []string
	}{
		{"clean run with a warning", nil, 0, nil},
		{"failed run keeps its failure lines", map[int]string{1200: "src/x.c:12: error: unknown type foo", 2400: "FAIL: link step"}, 2,
			[]string{"src/x.c:12: error: unknown type foo", "FAIL: link step"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rawOut := makeLog(t, 3000, tc.failAt, tc.code)
			store := &storeKeep{ok: true}
			var out, errb bytes.Buffer
			r := Run([]string{"make"}, Options{Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb,
				Compact: true, Keep: store.keep, Balanced: true})
			if r.ExitCode != tc.code || !r.Compacted || (tc.code != 0 && !r.Folded) {
				t.Fatalf("result %+v", r)
			}
			if errb.String() != "warning: deprecated flag\n" {
				t.Fatalf("stderr = %q", errb.String())
			}
			if len(store.got) != rawOut || out.Len()*4 > rawOut {
				t.Fatalf("kept %d of %d bytes, printed %d", len(store.got), rawOut, out.Len())
			}
			for _, w := range append(tc.want, "full output: downshift retrieve 0123456789ab]") {
				if !strings.Contains(out.String(), w) {
					t.Errorf("output lacks %q", w)
				}
			}
			if r.EmittedBytes != int64(out.Len()+errb.Len()) {
				t.Errorf("emitted=%d, printed %d", r.EmittedBytes, out.Len()+errb.Len())
			}
		})
	}
}

// Without a keep port a large failed make run with stderr stays raw, whole.
func TestRun_FailedMakeWithStderrWithoutKeepStaysRaw(t *testing.T) {
	rawOut := makeLog(t, 3000, nil, 1)
	var merged lockedBuf
	r := Run([]string{"make"}, Options{Stdin: strings.NewReader(""), Stdout: &merged, Stderr: &merged, Compact: true, Balanced: true})
	s := merged.b.String()
	if r.Compacted || len(s) != rawOut+len("warning: deprecated flag\n") {
		t.Fatalf("printed %d bytes, result %+v", len(s), r)
	}
	if strings.Count(s, "warning: deprecated flag\n") != 1 || r.Passthrough == "" {
		t.Fatalf("the warning is not printed once, or no reason: %+v", r)
	}
}

// buildLine is a line of a build log made of pseudo-random words of varied
// length, so no line shape repeats: only a head-and-tail cut shortens it.
func buildLine(i int) string {
	x := uint32(i)*2654435761 + 12345
	var b strings.Builder
	for k := 0; k < 6; k++ {
		x = x*1103515245 + 12345
		for n := 2 + int(x>>16)%7; n > 0; n-- {
			x = x*1103515245 + 12345
			b.WriteByte(byte('a' + (x>>16)%26))
		}
		b.WriteByte(' ')
	}
	return strings.TrimSpace(b.String())
}
