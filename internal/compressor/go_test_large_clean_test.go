// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

// goCoverRun builds a clean `go test -cover ./...` transcript over n packages.
// The module root has no test files, so it prints the tab-indented coverage
// line that go test emits for a package without tests.
func goCoverRun(n int) string {
	var b strings.Builder
	b.WriteString("\texample.com/many\t\tcoverage: 0.0% of statements\n")
	b.WriteString("?   \texample.com/many/nt\t[no test files]\n")
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "ok  \texample.com/many/p%d\t0.320s\tcoverage: 100.0%% of statements\n", i)
	}
	return b.String()
}

// A clean -cover run with a tab-indented no-test-files line must compact.
func TestCompress_GoTest_CoverNoTestFilesLineCompacts(t *testing.T) {
	in := []byte(goCoverRun(2))
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("clean -cover run must compact: reason=%q", res.Reason)
	}
	if !strings.HasPrefix(string(res.Output), "Go test: PASS (2 packages ok, 2 without tests)") {
		t.Errorf("unexpected summary: %q", res.Output)
	}
}

// A large clean -cover run (over the 10 KB the 2026-10-21 log flagged) compacts.
func TestCompress_GoTest_LargeCleanCoverRunCompacts(t *testing.T) {
	in := []byte(goCoverRun(200))
	if len(in) < 10*1024 {
		t.Fatalf("fixture too small: %d bytes", len(in))
	}
	res := compressor.Compress(in, compressor.ModeSafe)
	if !res.Applied {
		t.Fatalf("large clean -cover run must compact: reason=%q", res.Reason)
	}
	if len(res.Output) >= len(in) {
		t.Errorf("summary not smaller: %d >= %d", len(res.Output), len(in))
	}
}

// A failing line in a -cover run keeps the whole transcript byte-identical.
func TestCompress_GoTest_CoverRunWithFailurePreservesRaw(t *testing.T) {
	in := []byte(goCoverRun(3) + "--- FAIL: TestAdd2 (0.00s)\nFAIL\texample.com/many/p2\t0.2s\nFAIL\n")
	res := compressor.Compress(in, compressor.ModeSafe)
	if res.Applied || !bytes.Equal(res.Output, in) {
		t.Fatalf("failing -cover run must stay raw: applied=%v reason=%q", res.Applied, res.Reason)
	}
}

// Preserved by design: `go test -v` compaction covers one package. A
// multi-package -v run is not compacted, and a subtest transcript with a log
// line or a parallel marker is not either; that is pinned here so the limit
// stays a decision rather than an accident. (Passing, silent subtests do
// compact: see TestParseGoVerboseCompactsPassingSubtests.)
func TestCompress_GoVerbose_MultiPackageAndSubtestsPreservedByDesign(t *testing.T) {
	multi := "=== RUN   TestAdd1\n--- PASS: TestAdd1 (0.00s)\nPASS\nok  \texample.com/many/p1\t0.282s\n" +
		"=== RUN   TestAdd2\n--- PASS: TestAdd2 (0.00s)\nPASS\nok  \texample.com/many/p2\t0.281s\n"
	subtests := "=== RUN   TestAdd1\n=== RUN   TestAdd1/case\n    note from the subtest\n--- PASS: TestAdd1 (0.00s)\n    --- PASS: TestAdd1/case (0.00s)\nPASS\nok  \texample.com/many/p1\t0.282s\n"
	for name, raw := range map[string]string{"multi-package -v": multi, "subtests with a log line": subtests} {
		in := []byte(raw)
		res := compressor.Compress(in, compressor.ModeSafe)
		if res.Applied || !bytes.Equal(res.Output, in) {
			t.Errorf("%s: must stay raw by design: applied=%v reason=%q", name, res.Applied, res.Reason)
		}
	}
}
