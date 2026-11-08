// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/compressor"
)

func TestCompress_CargoTest_PassCompaction(t *testing.T) {
	in := []byte("   Compiling my_crate v0.1.0 (/home/dev/my_crate)\n    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.5s\n     Running unittests src/lib.rs (target/debug/deps/my_crate-1a2b3c)\n\nrunning 5 tests\ntest tests::test_1 ... ok\n\ntest result: ok. 5 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.15s\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("expected applied=true for passing cargo test: %s", res.Reason)
	}
	want := "Cargo test: PASS (5 passed in 0.15s)\n"
	if string(res.Output) != want {
		t.Errorf("got %q, want %q", string(res.Output), want)
	}
	if res.Format != compressor.FormatCargoTest {
		t.Errorf("expected format cargo_test, got %v", res.Format)
	}
}

// TestCompress_CargoTest_UnrecognisedLinePreservesRaw injects lines that a
// clean run never emits into an otherwise passing transcript. Each must keep
// the whole transcript byte-identical, at the start, middle and end.
func TestCompress_CargoTest_UnrecognisedLinePreservesRaw(t *testing.T) {
	clean := []string{
		"   Compiling my_crate v0.1.0 (/home/dev/my_crate)",
		"    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.5s",
		"     Running unittests src/lib.rs (target/debug/deps/my_crate-1a2b3c)",
		"",
		"running 2 tests",
		"test a::t1 ... ok",
		"test a::t2 ... ok",
		"",
		"test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.05s",
		"",
	}
	if res := compressor.Compress([]byte(strings.Join(clean, "\n")), compressor.ModeSafe); !res.Applied {
		t.Fatalf("clean transcript must still compact: %s", res.Reason)
	}
	injected := []string{
		"unexpected error in output",
		"unexpected warning in output",
		"unexpected panic in output",
		"unexpected FAIL in output",
	}
	for _, line := range injected {
		for pos := 0; pos <= len(clean); pos++ {
			lines := append(append(append([]string{}, clean[:pos]...), line), clean[pos:]...)
			in := []byte(strings.Join(lines, "\n"))
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("injected %q at %d must not compact: reason=%q", line, pos, res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("injected %q at %d must come back byte-identical", line, pos)
			}
		}
	}
}

// TestCompress_CargoTest_ErrorTextInCargoShapedLinePreservesRaw covers lines
// whose shape fits a cargo success pattern but whose free text carries an error
// word. Each must keep the transcript byte-identical.
func TestCompress_CargoTest_ErrorTextInCargoShapedLinePreservesRaw(t *testing.T) {
	clean := []string{
		"   Compiling my_crate v0.1.0 (/home/dev/my_crate)",
		"    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.5s",
		"     Running unittests src/lib.rs (target/debug/deps/my_crate-1a2b3c)",
		"",
		"running 2 tests",
		"test a::t1 ... ok",
		"test a::t2 ... ok",
		"",
		"test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.05s",
		"",
	}
	injected := []string{
		"   Compiling x v1 (error: x)",
		"     Running unittests x (error: boom)",
		"test x ... ok ... error: y ... ok",
	}
	for _, line := range injected {
		for pos := 0; pos <= len(clean); pos++ {
			lines := append(append(append([]string{}, clean[:pos]...), line), clean[pos:]...)
			in := []byte(strings.Join(lines, "\n"))
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("line %q at %d must not compact: reason=%q", line, pos, res.Reason)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("line %q at %d must come back byte-identical", line, pos)
			}
		}
	}
}

func TestCompress_CargoTest_ErrorsPreserveRaw(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"FAILED summary", "running 2 tests\ntest t1 ... ok\ntest t2 ... FAILED\n\ntest result: FAILED. 1 passed; 1 failed; finished in 0.05s\n"},
		{"compiler error E", "error[E0425]: cannot find value `foo` in this scope\n --> src/lib.rs:10:5\n"},
		{"compiler warning", "running 2 tests\ntest t1 ... ok\ntest t2 ... ok\nwarning: unused variable: `x`\n\ntest result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.05s\n"},
		{"panicked at", "thread 'test_panics' panicked at src/lib.rs:20:5:\nassertion failed\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := []byte(tc.in)
			res := compressor.Compress(in, compressor.ModeSafe)
			if res.Applied {
				t.Fatalf("must not compress cargo test with error: %s", tc.name)
			}
			if !bytes.Equal(res.Output, in) {
				t.Fatalf("must preserve raw output byte-identical")
			}
			if res.Format != compressor.FormatCargoTest {
				t.Errorf("expected format cargo_test, got %v", res.Format)
			}
		})
	}
}
