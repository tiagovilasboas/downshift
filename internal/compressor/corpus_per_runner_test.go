// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

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
