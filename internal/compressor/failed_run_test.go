// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

// sgrColour is a colour sequence; the test strips it to compare a coloured run
// with the plain text the compactor reads.
var sgrColour = regexp.MustCompile(`\x1b\[[0-9;:]*m`)

func readFailedFixture(t *testing.T, dir, name string) []byte {
	t.Helper()
	in, err := os.ReadFile(filepath.Join("testdata", dir, name+".in"))
	if err != nil {
		t.Fatal(err)
	}
	return in
}

// goldenFailedRun compacts one real fixture and compares it to its golden file.
func goldenFailedRun(t *testing.T, dir string, f compressor.Format, name string) (in []byte, r compressor.Result) {
	t.Helper()
	in = readFailedFixture(t, dir, name)
	r = compressor.CompressFailed(in, f)
	if !r.Applied || r.Format != f || len(r.Output) >= len(in) {
		t.Fatalf("not compacted: applied=%v format=%s reason=%q %d -> %d", r.Applied, r.Format, r.Reason, len(in), len(r.Output))
	}
	golden := filepath.Join("testdata", dir, name+".golden")
	if *updateFailedGolden {
		if err := os.WriteFile(golden, r.Output, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(r.Output, want) {
		t.Fatalf("output differs from golden\n got: %q\nwant: %q", r.Output, want)
	}
	return in, r
}

// assertOnlyNoiseCut checks the conformance rule of a failed run: after the
// one summary line, the output is the input (colour removed) in the same order
// with exactly the lines for which isNoise is true cut, so every other line,
// byte for byte, survived. isNoise is written here from the reporter's own
// pass markers, independently of the compactor's grammar.
func assertOnlyNoiseCut(t *testing.T, in, out []byte, summaryPrefix string, isNoise func(line string) bool) {
	t.Helper()
	outLines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if !strings.HasPrefix(outLines[0], summaryPrefix) {
		t.Fatalf("missing summary line %q: %q", summaryPrefix, outLines[0])
	}
	plain := sgrColour.ReplaceAllString(string(in), "")
	var want []string
	for _, l := range strings.Split(strings.TrimSuffix(plain, "\n"), "\n") {
		if !isNoise(l) {
			want = append(want, l)
		}
	}
	if strings.Join(want, "\n") != strings.Join(outLines[1:], "\n") {
		t.Fatalf("lines other than passing noise differ\n got: %q\nwant: %q", outLines[1:], want)
	}
}

// assertContains checks that named failure lines are in the output.
func assertContains(t *testing.T, out []byte, lines []string) {
	t.Helper()
	for _, l := range lines {
		if !strings.Contains(string(out), l+"\n") {
			t.Errorf("lost failure line %q", l)
		}
	}
}

// assertRaw checks that a run came back byte for byte with Applied false.
func assertRaw(t *testing.T, f compressor.Format, in []byte, wantReason string) {
	t.Helper()
	r := compressor.CompressFailed(in, f)
	if r.Applied || !bytes.Equal(r.Output, in) || r.ReducedBytes != len(in) {
		t.Fatalf("must fail open: applied=%v reason=%q", r.Applied, r.Reason)
	}
	if wantReason != "" && r.Reason != wantReason {
		t.Fatalf("reason = %q, want %q", r.Reason, wantReason)
	}
}

// mutate returns in with one literal replaced once; it fails when old is absent.
func mutate(t *testing.T, in []byte, old, repl string) []byte {
	t.Helper()
	if !bytes.Contains(in, []byte(old)) {
		t.Fatalf("fixture has no %q", old)
	}
	return bytes.Replace(in, []byte(old), []byte(repl), 1)
}

func TestCompressExitStillPreservesFailedVitestAndPlaywright(t *testing.T) {
	for dir, name := range map[string]string{"failed_vitest": "v4_fail", "failed_playwright": "list_fail"} {
		in := readFailedFixture(t, dir, name)
		if r := compressor.CompressExit(in, compressor.ModeSafe, 1); r.Applied || !bytes.Equal(r.Output, in) {
			t.Errorf("%s: CompressExit must keep preserving a non-zero exit", dir)
		}
	}
}
