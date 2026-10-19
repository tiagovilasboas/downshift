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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

const corpusDir = "testdata/corpus"

// nonZeroExits covers ordinary failures, timeout(1) and signal-style codes.
var nonZeroExits = []int{1, 2, 124, 130, 137, 255}

// rawFixtures must never change, whatever the exit code (including unknown -1).
var rawFixtures = []string{
	// go test: failures, panics, races, build and vet errors.
	"gotest_fail_mixed_ok.txt",
	"gotest_fail_mixed_no_test_files.txt",
	"gotest_fail_summary_only.txt",
	"gotest_panic.txt",
	"gotest_panic_mixed_no_test_files.txt",
	"gotest_race.txt",
	"gotest_race_no_fail_line.txt",
	"gotest_build_failed.txt",
	"gotest_vet_error.txt",
	// go test: not all-ok lines, so not clearly clean.
	"gotest_no_tests_to_run.txt",
	"gotest_verbose.txt",
	"gotest_pass_with_log_line.txt",
	"gotest_ansi.txt",
	"gotest_crlf.txt",
	"gotest_truncated_last_line.txt",
	// git status with anything to look at.
	"gitstatus_untracked.txt",
	"gitstatus_modified_staged.txt",
	"gitstatus_conflict.txt",
	"gitstatus_rebase_in_progress.txt",
	"gitstatus_detached_dirty.txt",
	"gitstatus_ansi.txt",
	"gitstatus_porcelain_v2.txt",
	"gitstatus_short.txt",
	// D1: repeated lines must not trigger log dedup on a failing run.
	"gotest_fail_repeated_log.txt",
	// D8: no final newline means possibly truncated.
	"gotest_pass_no_trailing_newline.txt",
	// D6: a summary may never be longer than the input.
	"gotest_two_short_pkgs.txt",
	// D3/D4: a clean tree still carries facts the summary would drop.
	"gitstatus_clean_ahead.txt",
	"gitstatus_clean_behind.txt",
	"gitstatus_clean_diverged.txt",
	"gitstatus_clean_upstream_gone.txt",
	"gitstatus_clean_bisecting.txt",
	"gitstatus_merge_concluding.txt",
	"gitstatus_clean_stash.txt",
	"gitstatus_clean_detached.txt",
	// D5: CRLF would be rewritten with a stray carriage return.
	"gitstatus_clean_crlf.txt",
	// git log is never summarised (D2: repeated body lines included).
	"gitlog_squash_repeated_body.txt",
	"gitlog_plain.txt",
	"gitlog_merge.txt",
	"gitlog_signed.txt",
	"gitlog_oneline.txt",
	"gitlog_body_mentions_fail.txt",
	// encodings.
	"non_utf8.bin",
	"binary_nul.bin",
	// pytest and vitest: failures and tracebacks stay raw.
	"pytest_fail.txt",
	"vitest_fail.txt",
	// playwright, cargo test and tsc: failures and errors stay raw.
	"playwright_fail.txt",
	"cargo_fail.txt",
	"tsc_fail.txt",
	"jest_fail.txt",
}

// cleanFixture is output that may be compacted on exit 0. facts are substrings
// that must survive compaction.
type cleanFixture struct {
	file  string
	facts []string
}

var cleanFixtures = []cleanFixture{
	{"gotest_pass_coverage.txt", []string{"PASS", "12 packages", "internal/billing", "65.9%", "cmd/widget", "100.0%"}},
	{"gotest_pass_cached.txt", []string{"PASS", "6 packages", "internal/config"}},
	{"gotest_pass_plain.txt", []string{"PASS", "12 packages", "internal/store"}},
	{"gotest_no_test_files.txt", []string{"PASS", "2 packages ok", "1 without tests", "cmd/widget", "internal/api", "internal/auth"}},
	{"gotest_pass_mixed_no_test_files.txt", []string{"PASS", "6 packages ok", "3 without tests", "cmd/dsmon-server", "internal/adapt", "80.5%", "internal/session", "internal/testrace"}},
	{"gitstatus_clean_uptodate.txt", []string{"On branch main", "working tree clean"}},
	{"gitstatus_clean_no_upstream.txt", []string{"On branch feature/rate-limit", "working tree clean"}},
	{"pytest_pass_clean.txt", []string{"25 passed"}},
	{"vitest_pass_clean.txt", []string{"25 passed"}},
	{"playwright_pass_clean.txt", []string{"25 passed"}},
	{"cargo_pass_clean.txt", []string{"25 passed"}},
	{"tsc_pass_clean.txt", []string{"0 errors"}},
	{"jest_pass_clean.txt", []string{"25 passed", "3 suites", "1 suite skipped", "1 todo"}},
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(corpusDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

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

// allFixtures returns every file in the corpus directory, so new fixtures are
// covered by the invariants without being listed.
func allFixtures(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	if len(out) < len(rawFixtures)+len(cleanFixtures) {
		t.Fatalf("corpus has %d files, fewer than the listed fixtures", len(out))
	}
	return out
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

// TestCorpus_Pytest verifies the pytest contract:
// - clean output with exit 0 is compacted and preserves core facts (25 passed).
// - clean output with exit != 0 passes through raw byte-a-byte.
// - failure output stays 100% byte-identical (raw) on exit 0, exit -1, or exit != 0.
func TestCorpus_Pytest(t *testing.T) {
	cleanIn := readFixture(t, "pytest_pass_clean.txt")
	failIn := readFixture(t, "pytest_fail.txt")
	codes := append([]int{-1, 0}, nonZeroExits...)

	// Clean exit 0 must compact
	t.Run("clean/exit=0", func(t *testing.T) {
		res := compressor.CompressExit(cleanIn, compressor.ModeSafe, 0)
		if !res.Applied {
			t.Fatalf("clean pytest not compacted: reason=%q", res.Reason)
		}
		if len(res.Output) >= len(cleanIn) {
			t.Fatalf("clean pytest did not shrink: %d -> %d", len(cleanIn), len(res.Output))
		}
		if !bytes.Contains(res.Output, []byte("25 passed")) {
			t.Fatalf("clean pytest lost '25 passed': %q", res.Output)
		}
	})

	// Clean exit != 0 must pass through raw
	for _, code := range nonZeroExits {
		t.Run(fmt.Sprintf("clean/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(cleanIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, cleanIn) || res.Reason != "nonzero_exit_preserved" {
				t.Fatalf("clean pytest exit=%d not raw: applied=%v reason=%q", code, res.Applied, res.Reason)
			}
		})
	}

	// Fail cases must stay byte-identical raw across all exit codes (-1, 0, nonZeroExits)
	for _, code := range codes {
		t.Run(fmt.Sprintf("fail/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(failIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, failIn) {
				t.Fatalf("failing pytest modified: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}

// TestCorpus_Vitest verifies the vitest contract:
// - clean output with exit 0 is compacted and preserves core facts (25 passed).
// - clean output with exit != 0 passes through raw byte-a-byte.
// - failure output stays 100% byte-identical (raw) on exit 0, exit -1, or exit != 0.
func TestCorpus_Vitest(t *testing.T) {
	cleanIn := readFixture(t, "vitest_pass_clean.txt")
	failIn := readFixture(t, "vitest_fail.txt")
	codes := append([]int{-1, 0}, nonZeroExits...)

	// Clean exit 0 must compact
	t.Run("clean/exit=0", func(t *testing.T) {
		res := compressor.CompressExit(cleanIn, compressor.ModeSafe, 0)
		if !res.Applied {
			t.Fatalf("clean vitest not compacted: reason=%q", res.Reason)
		}
		if len(res.Output) >= len(cleanIn) {
			t.Fatalf("clean vitest did not shrink: %d -> %d", len(cleanIn), len(res.Output))
		}
		if !bytes.Contains(res.Output, []byte("25 passed")) {
			t.Fatalf("clean vitest lost '25 passed': %q", res.Output)
		}
	})

	// Clean exit != 0 must pass through raw
	for _, code := range nonZeroExits {
		t.Run(fmt.Sprintf("clean/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(cleanIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, cleanIn) || res.Reason != "nonzero_exit_preserved" {
				t.Fatalf("clean vitest exit=%d not raw: applied=%v reason=%q", code, res.Applied, res.Reason)
			}
		})
	}

	// Fail cases must stay byte-identical raw across all exit codes (-1, 0, nonZeroExits)
	for _, code := range codes {
		t.Run(fmt.Sprintf("fail/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(failIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, failIn) {
				t.Fatalf("failing vitest modified: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}

// TestCorpus_Playwright verifies the playwright contract:
// - clean output with exit 0 is compacted and preserves core facts (25 passed).
// - clean output with exit != 0 passes through raw byte-a-byte.
// - failure output stays 100% byte-identical (raw) on exit 0, exit -1, or exit != 0.
func TestCorpus_Playwright(t *testing.T) {
	cleanIn := readFixture(t, "playwright_pass_clean.txt")
	failIn := readFixture(t, "playwright_fail.txt")
	codes := append([]int{-1, 0}, nonZeroExits...)

	// Clean exit 0 must compact
	t.Run("clean/exit=0", func(t *testing.T) {
		res := compressor.CompressExit(cleanIn, compressor.ModeSafe, 0)
		if !res.Applied {
			t.Fatalf("clean playwright not compacted: reason=%q", res.Reason)
		}
		if len(res.Output) >= len(cleanIn) {
			t.Fatalf("clean playwright did not shrink: %d -> %d", len(cleanIn), len(res.Output))
		}
		if !bytes.Contains(res.Output, []byte("25 passed")) {
			t.Fatalf("clean playwright lost '25 passed': %q", res.Output)
		}
	})

	// Clean exit != 0 must pass through raw
	for _, code := range nonZeroExits {
		t.Run(fmt.Sprintf("clean/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(cleanIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, cleanIn) || res.Reason != "nonzero_exit_preserved" {
				t.Fatalf("clean playwright exit=%d not raw: applied=%v reason=%q", code, res.Applied, res.Reason)
			}
		})
	}

	// Fail cases must stay byte-identical raw across all exit codes (-1, 0, nonZeroExits)
	for _, code := range codes {
		t.Run(fmt.Sprintf("fail/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(failIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, failIn) {
				t.Fatalf("failing playwright modified: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}

// TestCorpus_CargoTest verifies the cargo test contract:
// - clean output with exit 0 is compacted and preserves core facts (25 passed).
// - clean output with exit != 0 passes through raw byte-a-byte.
// - failure output stays 100% byte-identical (raw) on exit 0, exit -1, or exit != 0.
func TestCorpus_CargoTest(t *testing.T) {
	cleanIn := readFixture(t, "cargo_pass_clean.txt")
	failIn := readFixture(t, "cargo_fail.txt")
	codes := append([]int{-1, 0}, nonZeroExits...)

	// Clean exit 0 must compact
	t.Run("clean/exit=0", func(t *testing.T) {
		res := compressor.CompressExit(cleanIn, compressor.ModeSafe, 0)
		if !res.Applied {
			t.Fatalf("clean cargo test not compacted: reason=%q", res.Reason)
		}
		if len(res.Output) >= len(cleanIn) {
			t.Fatalf("clean cargo test did not shrink: %d -> %d", len(cleanIn), len(res.Output))
		}
		if !bytes.Contains(res.Output, []byte("25 passed")) {
			t.Fatalf("clean cargo test lost '25 passed': %q", res.Output)
		}
	})

	// Clean exit != 0 must pass through raw
	for _, code := range nonZeroExits {
		t.Run(fmt.Sprintf("clean/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(cleanIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, cleanIn) || res.Reason != "nonzero_exit_preserved" {
				t.Fatalf("clean cargo test exit=%d not raw: applied=%v reason=%q", code, res.Applied, res.Reason)
			}
		})
	}

	// Fail cases must stay byte-identical raw across all exit codes (-1, 0, nonZeroExits)
	for _, code := range codes {
		t.Run(fmt.Sprintf("fail/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(failIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, failIn) {
				t.Fatalf("failing cargo test modified: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}

// TestCorpus_TSC verifies the tsc contract:
// - clean output with exit 0 is compacted and preserves core facts (0 errors).
// - clean output with exit != 0 passes through raw byte-a-byte.
// - failure output stays 100% byte-identical (raw) on exit 0, exit -1, or exit != 0.
func TestCorpus_TSC(t *testing.T) {
	cleanIn := readFixture(t, "tsc_pass_clean.txt")
	failIn := readFixture(t, "tsc_fail.txt")
	codes := append([]int{-1, 0}, nonZeroExits...)

	// Clean exit 0 must compact
	t.Run("clean/exit=0", func(t *testing.T) {
		res := compressor.CompressExit(cleanIn, compressor.ModeSafe, 0)
		if !res.Applied {
			t.Fatalf("clean tsc not compacted: reason=%q", res.Reason)
		}
		if len(res.Output) >= len(cleanIn) {
			t.Fatalf("clean tsc did not shrink: %d -> %d", len(cleanIn), len(res.Output))
		}
		if !bytes.Contains(res.Output, []byte("0 errors")) {
			t.Fatalf("clean tsc lost '0 errors': %q", res.Output)
		}
	})

	// Clean exit != 0 must pass through raw
	for _, code := range nonZeroExits {
		t.Run(fmt.Sprintf("clean/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(cleanIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, cleanIn) || res.Reason != "nonzero_exit_preserved" {
				t.Fatalf("clean tsc exit=%d not raw: applied=%v reason=%q", code, res.Applied, res.Reason)
			}
		})
	}

	// Fail cases must stay byte-identical raw across all exit codes (-1, 0, nonZeroExits)
	for _, code := range codes {
		t.Run(fmt.Sprintf("fail/exit=%d", code), func(t *testing.T) {
			res := compressor.CompressExit(failIn, compressor.ModeSafe, code)
			if res.Applied || !bytes.Equal(res.Output, failIn) {
				t.Fatalf("failing tsc modified: applied=%v reason=%q", res.Applied, res.Reason)
			}
		})
	}
}
