// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestCompress_Jest_PassCompaction(t *testing.T) {
	in := []byte("PASS src/auth.test.ts\nPASS src/billing.test.ts\n\nTest Suites: 1 skipped, 2 passed, 3 total\nTests:       1 skipped, 1 todo, 25 passed, 27 total\nSnapshots:   0 total\nTime:        1.234 s\nRan all test suites.\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing Jest: %s", res.Reason)
	}
	want := "Jest: PASS (1 skipped, 1 todo, 25 passed across 3 suites, 1 suite skipped)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatJest {
		t.Errorf("expected format jest, got %v", res.Format)
	}
}

func TestCompress_Jest_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"failing suite", "FAIL src/auth.test.ts\n\nTest Suites: 1 failed, 1 passed, 2 total\nTests: 1 failed, 25 passed, 26 total\n"},
		{"failing summary", "Test Suites: 1 passed, 1 total\nTests: 1 failed, 24 passed, 25 total\n"},
		{"error output", "PASS src/auth.test.ts\nError: worker crashed\nTest Suites: 1 passed, 1 total\nTests: 25 passed, 25 total\n"},
		{"incomplete summary", "PASS src/auth.test.ts\nTest Suites: 1 passed, 1 total\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress unsafe Jest output: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatal("must preserve raw output byte-identical")
			}
			if res.Format != compressor.FormatJest {
				t.Errorf("expected format jest, got %v", res.Format)
			}
		})
	}
}

func TestCompress_Jest_KeepsFewConsoleLinesVerbatim(t *testing.T) {
	in := []byte("PASS src/auth.test.ts\nconsole.log CUSTOMER_ID=12345\nTest Suites: 1 passed, 1 total\nTests: 25 passed, 25 total\n")
	res := compressor.CompressExit(in, compressor.ModeSafe, 0)
	want := "Jest: PASS (25 passed across 1 suites)\nconsole.log CUSTOMER_ID=12345\n"
	if !res.Applied || string(res.Output) != want {
		t.Fatalf("a few console lines must stay verbatim: applied=%v output=%q", res.Applied, res.Output)
	}
}

func TestCompress_Jest_DropsManyConsoleLinesWithCount(t *testing.T) {
	in := "PASS src/auth.test.ts\n" + strings.Repeat("console.log CUSTOMER_ID=12345\n", 21) +
		"Coverage | 91.2 % | x\nTest Suites: 1 passed, 1 total\nTests: 25 passed, 25 total\n"
	res := compressor.CompressExit([]byte(in), compressor.ModeSafe, 0)
	want := "Jest: PASS (25 passed across 1 suites)\nCoverage | 91.2 % | x\n[21 console output lines dropped]\n"
	if !res.Applied || string(res.Output) != want {
		t.Fatalf("many console lines fold into a counted marker: applied=%v output=%q", res.Applied, res.Output)
	}
}

func TestCompress_Jest_CleanRunSummary(t *testing.T) {
	in := "PASS src/a.test.ts\nPASS src/b.test.ts\nPASS src/c.test.ts\n\nTest Suites: 3 passed, 3 total\nTests:       12 passed, 12 total\nSnapshots:   0 total\nTime:        1.234 s\nRan all test suites.\n"
	res := compressor.CompressExit([]byte(in), compressor.ModeSafe, 0)
	if !res.Applied || res.Format != compressor.FormatJest {
		t.Fatalf("clean Jest run should compact: %+v", res)
	}
	if got, want := string(res.Output), "Jest: PASS (12 passed across 3 suites)\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
