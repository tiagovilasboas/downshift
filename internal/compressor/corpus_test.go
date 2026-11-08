// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

// Adversarial regression corpus for the compressor on the token-cutting path
// (downshift exec). Fixtures under testdata/corpus are synthetic.
//
// Contract checked here:
//   (a) output that is not clearly clean stays byte-identical, for every exit code;
//   (b) clean output is compacted and the facts that matter survive;
//   (c) compaction is idempotent and never grows the output;
//   (d) a non-zero exit code always passes the output through raw.
//
//   (e) every defect QA found in the first corpus (D1-D8) stays fixed.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestCorpusRawStaysByteIdentical(t *testing.T) {
	exits := append([]int{-1, 0}, nonZeroExits...)
	for _, name := range rawFixtures {
		in := readFixture(t, name)
		for _, code := range exits {
			t.Run(fmt.Sprintf("%s/exit=%d", name, code), func(t *testing.T) {
				res := compressor.CompressExit(in, compressor.ModeSafe, code)
				if res.Applied || !bytes.Equal(res.Output, in) {
					t.Fatalf("raw fixture changed: applied=%v reason=%q\n got: %q", res.Applied, res.Reason, res.Output)
				}
			})
		}
	}
}

func TestCorpusCleanCompactsAndKeepsFacts(t *testing.T) {
	for _, c := range cleanFixtures {
		t.Run(c.file, func(t *testing.T) {
			in := readFixture(t, c.file)
			res := compressor.CompressExit(in, compressor.ModeSafe, 0)
			if !res.Applied {
				t.Fatalf("clean fixture not compacted: reason=%q", res.Reason)
			}
			if len(res.Output) >= len(in) {
				t.Fatalf("compaction did not shrink: %d -> %d", len(in), len(res.Output))
			}
			if res.ReducedBytes != len(res.Output) || res.OriginalBytes != len(in) {
				t.Fatalf("byte accounting: original=%d reduced=%d, want %d/%d",
					res.OriginalBytes, res.ReducedBytes, len(in), len(res.Output))
			}
			for _, f := range c.facts {
				if !bytes.Contains(res.Output, []byte(f)) {
					t.Errorf("fact %q lost; output %q", f, res.Output)
				}
			}
		})
	}
}

func TestCorpusNonZeroExitPassesThroughRaw(t *testing.T) {
	for _, c := range cleanFixtures {
		in := readFixture(t, c.file)
		for _, code := range nonZeroExits {
			t.Run(fmt.Sprintf("%s/exit=%d", c.file, code), func(t *testing.T) {
				res := compressor.CompressExit(in, compressor.ModeSafe, code)
				if res.Applied || !bytes.Equal(res.Output, in) || res.Reason != "nonzero_exit_preserved" {
					t.Fatalf("non-zero exit not raw: applied=%v reason=%q", res.Applied, res.Reason)
				}
			})
		}
	}
}

func TestCorpusIdempotentAndNeverGrows(t *testing.T) {
	for _, name := range allFixtures(t) {
		in := readFixture(t, name)
		for _, code := range []int{-1, 0} {
			t.Run(fmt.Sprintf("%s/exit=%d", name, code), func(t *testing.T) {
				once := compressor.CompressExit(in, compressor.ModeSafe, code)
				if len(once.Output) > len(in) {
					t.Fatalf("output grew: %d -> %d", len(in), len(once.Output))
				}
				twice := compressor.CompressExit(once.Output, compressor.ModeSafe, code)
				if !bytes.Equal(twice.Output, once.Output) {
					t.Fatalf("not idempotent:\n once: %q\ntwice: %q", once.Output, twice.Output)
				}
			})
		}
	}
}

func TestCorpusOffAndObserveNeverModify(t *testing.T) {
	for _, name := range allFixtures(t) {
		in := readFixture(t, name)
		for _, mode := range []compressor.Mode{compressor.ModeOff, compressor.ModeObserve} {
			res := compressor.CompressExit(in, mode, 0)
			if !bytes.Equal(res.Output, in) {
				t.Errorf("%s mode=%s modified output", name, mode)
			}
		}
	}
}

func TestCorpusOverMemoryBoundStaysRaw(t *testing.T) {
	line := []byte("ok  \texample.com/acme/widget/internal/api\t0.212s\n")
	in := bytes.Repeat(line, compressor.MaxProcessableBytes/len(line)+2)
	if len(in) <= compressor.MaxProcessableBytes {
		t.Fatalf("fixture too small: %d", len(in))
	}
	for _, code := range []int{-1, 0} {
		res := compressor.CompressExit(in, compressor.ModeSafe, code)
		if res.Applied || !bytes.Equal(res.Output, in) || res.Reason != "input_exceeds_memory_bound" {
			t.Fatalf("exit=%d >2MB input changed: applied=%v reason=%q", code, res.Applied, res.Reason)
		}
	}
	// Just under the bound, the same clean suite is compacted.
	under := in[:compressor.MaxProcessableBytes-compressor.MaxProcessableBytes%len(line)]
	if res := compressor.CompressExit(under, compressor.ModeSafe, 0); !res.Applied {
		t.Fatalf("under-bound clean suite not compacted: %q", res.Reason)
	}
}

// TestCorpusCleanReduction reports the byte reduction on clean fixtures; this
// is the number that says whether downshift exec pays for itself.
func TestCorpusCleanReduction(t *testing.T) {
	var before, after int
	for _, c := range cleanFixtures {
		in := readFixture(t, c.file)
		res := compressor.CompressExit(in, compressor.ModeSafe, 0)
		before += len(in)
		after += len(res.Output)
		t.Logf("%-34s %5d -> %4d bytes (%.1f%%)", c.file, len(in), len(res.Output), res.SavingsRatio())
	}
	if before == 0 || after >= before {
		t.Fatalf("no reduction on clean corpus: %d -> %d", before, after)
	}
	t.Logf("%-34s %5d -> %4d bytes (%.1f%%)", "TOTAL", before, after, float64(before-after)/float64(before)*100)
}

// TestCorpus_AdversarialGoTestFailOpenWithNoTestFiles verifies that suites mixing
// failures (--- FAIL: or panic:) with [no test files] packages stay 100% raw (fail-open),
// even if exit code is 0, -1, or non-zero.
func TestCorpus_AdversarialGoTestFailOpenWithNoTestFiles(t *testing.T) {
	failIn := readFixture(t, "gotest_fail_mixed_no_test_files.txt")
	panicIn := readFixture(t, "gotest_panic_mixed_no_test_files.txt")
	codes := append([]int{-1, 0}, nonZeroExits...)

	for _, code := range codes {
		t.Run(fmt.Sprintf("fail/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(failIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, failIn) {
				t.Fatalf("failing test with no test files modified: applied=%v reason=%q", res.Applied, res.Reason)
			}
			if code <= 0 && res.Reason != "error_detected_fail_open_raw_preserved" {
				t.Fatalf("exit=%d: reason=%q, want error_detected_fail_open_raw_preserved", code, res.Reason)
			}
		})

		t.Run(fmt.Sprintf("panic/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(panicIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, panicIn) {
				t.Fatalf("panic test with no test files modified: applied=%v reason=%q", res.Applied, res.Reason)
			}
			if code <= 0 && res.Reason != "error_detected_fail_open_raw_preserved" {
				t.Fatalf("exit=%d: reason=%q, want error_detected_fail_open_raw_preserved", code, res.Reason)
			}
		})
	}
}
