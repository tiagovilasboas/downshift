// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

const cargoSummary = "Cargo test: "

// reCargoPassNoise is written here, independently of the compactor, from what
// cargo prints for a crate that passed: its header, blank lines and counts.
var reCargoPassNoise = regexp.MustCompile(`^$|^\s+Running |^\s+Doc-tests |^running \d+ tests?$|^test .* \.\.\. (?:ok|ignored.*)$|^test result: ok\. `)

// assertCargoOnlyPassingCut checks the conformance rule: the output after the
// summary line is a subsequence of the input, and every line that was cut is
// passing noise, so no failure, warning or error line was lost.
func assertCargoOnlyPassingCut(t *testing.T, in, out []byte) {
	t.Helper()
	outLines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if !strings.HasPrefix(outLines[0], cargoSummary) {
		t.Fatalf("missing summary line: %q", outLines[0])
	}
	i := 0
	for _, l := range strings.Split(strings.TrimSuffix(string(in), "\n"), "\n") {
		if i+1 < len(outLines) && outLines[i+1] == l {
			i++
			continue
		}
		if !reCargoPassNoise.MatchString(l) {
			t.Errorf("cut a line that is not passing noise: %q", l)
		}
	}
	if i != len(outLines)-1 {
		t.Fatalf("output is not an ordered subset of the input (matched %d of %d)", i, len(outLines)-1)
	}
}

func cargoFixtureCut(t *testing.T, file string) (in []byte, r compressor.Result) {
	t.Helper()
	in = readFixture(t, file)
	r = compressor.CompressFailed(in, compressor.FormatCargoTest)
	if !r.Applied || r.Format != compressor.FormatCargoTest || len(r.Output) >= len(in) {
		t.Fatalf("not compacted: applied=%v reason=%q %d -> %d", r.Applied, r.Reason, len(in), len(r.Output))
	}
	t.Logf("%s: %d -> %d bytes (%.1f%% saved)", file, len(in), len(r.Output), 100*float64(len(in)-len(r.Output))/float64(len(in)))
	assertCargoOnlyPassingCut(t, in, r.Output)
	return in, r
}

func TestCompressFailedCargoLibOnly(t *testing.T) {
	_, r := cargoFixtureCut(t, "cargo_failed_lib_only.txt")
	if !strings.HasPrefix(string(r.Output), "Cargo test: 0 passing crates, 22 passing test lines cut") {
		t.Fatalf("summary: %q", strings.SplitN(string(r.Output), "\n", 2)[0])
	}
	assertContains(t, r.Output, []string{
		"test billing::tests::invoice_total_with_discount ... FAILED",
		"test tax::tests::us_sales_tax_by_state ... ignored, needs network",
		"---- billing::tests::invoice_total_with_discount stdout ----",
		"thread 'billing::tests::invoice_total_with_discount' panicked at src/billing.rs:118:9:",
		"assertion `left == right` failed: discount applied twice",
		"  left: 9000",
		" right: 9500",
		"failures:",
		"    billing::tests::invoice_total_with_discount",
		"test result: FAILED. 22 passed; 1 failed; 1 ignored; 0 measured; 0 filtered out; finished in 0.04s",
		"error: test failed, to rerun pass `--lib`",
	})
}

func TestCompressFailedCargoMultiCrate(t *testing.T) {
	_, r := cargoFixtureCut(t, "cargo_failed_multi_crate.txt")
	if !strings.HasPrefix(string(r.Output), "Cargo test: 4 passing crates, 20 passing test lines cut") {
		t.Fatalf("summary: %q", strings.SplitN(string(r.Output), "\n", 2)[0])
	}
	out := string(r.Output)
	for _, gone := range []string{"Running unittests src/lib.rs", "Running unittests src/main.rs", "Running tests/ledger.rs", "Doc-tests billing_core", "test result: ok."} {
		if strings.Contains(out, gone) {
			t.Errorf("passing crate line kept: %q", gone)
		}
	}
	assertContains(t, r.Output, []string{
		"warning: unused variable: `rate`",
		"     Running tests/api.rs (target/debug/deps/api-9c8b7a6f5e4d3c2b)",
		"test refund_twice_is_conflict ... FAILED",
		"thread 'refund_twice_is_conflict' panicked at tests/api.rs:204:5:",
		"expected status 409, got 200",
		"   2: api::refund_twice_is_conflict",
		"test result: FAILED. 5 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.31s",
		"error: 1 target failed:",
		"    `--test api`",
	})
}

func TestCompressFailedCargoFailsOpen(t *testing.T) {
	lib := readFixture(t, "cargo_failed_lib_only.txt")
	multi := readFixture(t, "cargo_failed_multi_crate.txt")
	t.Run("CRLF", func(t *testing.T) {
		assertRaw(t, compressor.FormatCargoTest, bytes.ReplaceAll(lib, []byte("\n"), []byte("\r\n")), "cargo_test_failed_run_raw_preserved")
	})
	t.Run("missing final newline", func(t *testing.T) {
		assertRaw(t, compressor.FormatCargoTest, bytes.TrimSuffix(lib, []byte("\n")), "cargo_test_failed_run_raw_preserved")
	})
	t.Run("colour escape", func(t *testing.T) {
		assertRaw(t, compressor.FormatCargoTest, mutate(t, lib, "running 24 tests", "\x1b[32mrunning 24 tests"), "cargo_test_failed_run_raw_preserved")
	})
	t.Run("no failure signal", func(t *testing.T) {
		compileError := []byte("   Compiling x v0.1.0 (/p)\nerror[E0425]: cannot find value `y` in this scope\n --> src/lib.rs:3:5\n\nerror: could not compile `x` (lib test) due to 1 previous error\n")
		assertRaw(t, compressor.FormatCargoTest, compileError, "cargo_test_failed_run_raw_preserved")
	})
	t.Run("nothing to cut", func(t *testing.T) {
		only := "     Running unittests src/lib.rs (target/debug/deps/x-1)\n\nrunning 1 test\ntest a ... FAILED\n\nfailures:\n\n---- a stdout ----\nthread 'a' panicked at src/lib.rs:1:1:\nboom\n\nfailures:\n    a\n\ntest result: FAILED. 0 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.00s\n\n"
		assertRaw(t, compressor.FormatCargoTest, []byte(only), "cargo_test_failed_run_no_gain")
	})
	t.Run("passed count disagrees keeps the crate", func(t *testing.T) {
		// A passing crate whose result line does not match its test lines is not cut.
		odd := mutate(t, multi, "test result: ok. 5 passed; 0 failed", "test result: ok. 6 passed; 0 failed")
		r := compressor.CompressFailed(odd, compressor.FormatCargoTest)
		if !strings.Contains(string(r.Output), "test result: ok. 6 passed; 0 failed") || !strings.Contains(string(r.Output), "test append_entry_is_ordered ... ok") {
			t.Fatalf("crate with disagreeing counts was cut")
		}
		assertCargoOnlyPassingCut(t, odd, r.Output)
	})
	t.Run("unrecognised line inside a passing crate keeps the crate", func(t *testing.T) {
		odd := mutate(t, multi, "test replay_from_snapshot ... ok\n", "test replay_from_snapshot ... ok\ndebug: leaked println\n")
		r := compressor.CompressFailed(odd, compressor.FormatCargoTest)
		if !strings.Contains(string(r.Output), "debug: leaked println\n") {
			t.Fatalf("lost an unrecognised line")
		}
		assertCargoOnlyPassingCut(t, odd, r.Output)
	})
	t.Run("interleaved output in a failing crate keeps the crate", func(t *testing.T) {
		// With --nocapture a test prints between the test lines: the count
		// of test lines no longer matches "running N tests", so nothing is cut.
		odd := mutate(t, lib, "test auth::tests::accepts_valid_token ... ok\n", "test auth::tests::accepts_valid_token ... ok\nhello from a test\n")
		r := compressor.CompressFailed(odd, compressor.FormatCargoTest)
		if !strings.Contains(string(r.Output), "test auth::tests::rejects_expired_token ... ok\n") || !strings.Contains(string(r.Output), "hello from a test\n") {
			t.Fatalf("cut inside an unparsed block")
		}
	})
	t.Run("failed count disagrees keeps the crate", func(t *testing.T) {
		odd := mutate(t, lib, "22 passed; 1 failed", "22 passed; 2 failed")
		r := compressor.CompressFailed(odd, compressor.FormatCargoTest)
		if r.Applied {
			t.Fatalf("a crate with disagreeing counts must stay raw")
		}
	})
}
