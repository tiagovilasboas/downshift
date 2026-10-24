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

func foldRun(t *testing.T, in string) compressor.Result {
	t.Helper()
	return compressor.CompressExit([]byte(in), compressor.ModeSafe, 0)
}

func mustFold(t *testing.T, in string, want ...string) string {
	t.Helper()
	res := foldRun(t, in)
	if !res.Applied {
		t.Fatalf("expected fold, reason=%q", res.Reason)
	}
	if len(res.Output) >= len(in) {
		t.Fatalf("not smaller: %d >= %d", len(res.Output), len(in))
	}
	for _, w := range want {
		if !strings.Contains(string(res.Output), w) {
			t.Errorf("output lost %q:\n%s", w, res.Output)
		}
	}
	return string(res.Output)
}

func mustStayRaw(t *testing.T, in string) {
	t.Helper()
	res := foldRun(t, in)
	if res.Applied || !bytes.Equal(res.Output, []byte(in)) {
		t.Fatalf("must stay raw: applied=%v reason=%q", res.Applied, res.Reason)
	}
}

// verboseLogged builds one -v package whose n passed tests each log a line.
func verboseLogged(n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "=== RUN   TestCase%d\n    case_test.go:12: warm 1.2s\n--- PASS: TestCase%d (0.00s)\n", i, i)
	}
	b.WriteString("PASS\nok  \texample.com/svc/pkg\t0.412s\n")
	return b.String()
}

// (a) passed tests that logged output fold into the counts.
func TestFold_VerbosePassedLogsFold(t *testing.T) {
	in := verboseLogged(40)
	out := mustFold(t, in, "40 passed", "40 passed tests logged output (folded)", "example.com/svc/pkg")
	if strings.Contains(out, "warm 1.2s") {
		t.Errorf("folded log lines must not survive:\n%s", out)
	}
	t.Logf("verbose logged: %d -> %d bytes", len(in), len(out))
}

// PAUSE/CONT markers of a passed parallel test fold without counting as logs.
func TestFold_VerbosePauseContFolds(t *testing.T) {
	in := "=== RUN   TestA\n=== PAUSE TestA\n=== CONT  TestA\n--- PASS: TestA (0.00s)\n" +
		"=== RUN   TestB\n=== PAUSE TestB\n=== CONT  TestB\n--- PASS: TestB (0.00s)\nPASS\nok  \texample.com/p\t0.1s\n"
	out := mustFold(t, in, "2 passed")
	if strings.Contains(out, "logged output") {
		t.Errorf("markers are not logs:\n%s", out)
	}
}

// A block that is not clearly a passed test stays verbatim while others fold.
func TestFold_VerboseUnrecognisedBlockKept(t *testing.T) {
	in := verboseLogged(30)
	in = "=== RUN   TestOdd\nstdout noise from the test\n--- PASS: TestOdd (0.00s)\n" + in
	out := mustFold(t, in, "Kept verbatim:", "stdout noise from the test", "=== RUN   TestOdd", "31 passed")
	if strings.Contains(out, "warm 1.2s") {
		t.Errorf("clean logged blocks should fold:\n%s", out)
	}
}

// A skipped test that logged its reason keeps the block; a log line carrying a
// danger word keeps its block verbatim as well.
func TestFold_VerboseSkipAndDangerBlocksKept(t *testing.T) {
	in := verboseLogged(30)
	in = "=== RUN   TestSkipNet\n    net_test.go:9: needs network\n--- SKIP: TestSkipNet (0.00s)\n" +
		"=== RUN   TestRetry\n    retry_test.go:3: retry after timeout ok\n--- PASS: TestRetry (0.00s)\n" + in
	mustFold(t, in, "net_test.go:9: needs network", "retry after timeout ok", "=== RUN   TestRetry")
}

// FAIL in a -v run is never folded.
func TestFold_VerboseFailStaysRaw(t *testing.T) {
	in := verboseLogged(10)
	in = strings.Replace(in, "--- PASS: TestCase3 (0.00s)", "--- FAIL: TestCase3 (0.00s)", 1)
	in = strings.Replace(in, "PASS\nok ", "FAIL\nFAIL", 1)
	mustStayRaw(t, in)
}

// A stray line with a failure signal anywhere in a -v run keeps it raw.
func TestFold_VerboseStrayDangerStaysRaw(t *testing.T) {
	mustStayRaw(t, "panic: boom in init\n"+verboseLogged(10))
	mustStayRaw(t, "WARNING: DATA RACE\n"+verboseLogged(10))
}

// A stray non-test line in a -v run is kept verbatim.
func TestFold_VerboseStrayLineKept(t *testing.T) {
	mustFold(t, "hello from init\n"+verboseLogged(30), "hello from init", "30 passed")
}

func plainRun(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "ok  \texample.com/svc/p%d\t0.320s\n", i)
	}
	return b.String()
}

// (b) stray lines are kept verbatim, in order; ok lines still fold.
func TestFold_PlainStrayLinesKeptInOrder(t *testing.T) {
	in := "hello from init\n" + plainRun(3) + "2026/10/19 10:00:00 WARN deprecated flag used\n" + plainRun(2)
	out := mustFold(t, in, "5 packages ok", "hello from init", "WARN deprecated flag used")
	if strings.Index(out, "hello from init") > strings.Index(out, "WARN deprecated") {
		t.Errorf("stray lines out of order:\n%s", out)
	}
	t.Logf("plain stray: %d -> %d bytes", len(in), len(out))
}

// A stray line with a danger word or a build header keeps the output raw.
func TestFold_PlainDangerStrayStaysRaw(t *testing.T) {
	for _, l := range []string{"panic: oops", "# example.com/svc/p1", "WARNING: DATA RACE", "context deadline: timeout exceeded"} {
		mustStayRaw(t, l+"\n"+plainRun(4))
	}
}

// (c) -cover with a single package folds when the output has several lines.
func TestFold_PlainCoverSinglePackage(t *testing.T) {
	in := "PASS\ncoverage: 71.4% of statements\nok  \texample.com/svc/p1\t0.320s\tcoverage: 71.4% of statements\n"
	out := mustFold(t, in, "1 packages ok", "ok p1 71.4%")
	t.Logf("cover single: %d -> %d bytes", len(in), len(out))
}

// One lone ok line has nothing to fold.
func TestFold_PlainSingleLineStaysRaw(t *testing.T) {
	mustStayRaw(t, "ok  \texample.com/svc/p1\t0.320s\n")
}

// A larger -cover run states min and max up front and keeps each figure.
func TestFold_PlainCoverMinMax(t *testing.T) {
	in := "ok  \tx/a\t0.1s\tcoverage: 59.2% of statements\nok  \tx/b\t0.1s\tcoverage: 100.0% of statements\n" +
		"ok  \tx/c\t0.1s\tcoverage: 80.0% of statements\nok  \tx/d\t0.1s\tcoverage: 90.0% of statements\n"
	mustFold(t, in, "coverage min 59.2% max 100.0%", "ok a 59.2%", "ok b 100.0%")
}

// Mixed (a)+(b): a -v run with logged tests and a stray init line.
func TestFold_VerboseMixed(t *testing.T) {
	in := "hello from init\n" + verboseLogged(25)
	out := mustFold(t, in, "hello from init", "25 passed tests logged output (folded)")
	if strings.Contains(out, "warm 1.2s") {
		t.Errorf("logs must fold:\n%s", out)
	}
}

// The summary fed back in is left alone.
func TestFold_Idempotent(t *testing.T) {
	res := foldRun(t, "hello from init\n"+plainRun(5))
	if !res.Applied {
		t.Fatalf("setup: %q", res.Reason)
	}
	mustStayRaw(t, string(res.Output))
	v := foldRun(t, verboseLogged(20))
	mustStayRaw(t, string(v.Output))
}

// -json output stays raw.
func TestFold_JSONStaysRaw(t *testing.T) {
	mustStayRaw(t, `{"Action":"run","Test":"TestA"}`+"\n"+`{"Action":"pass","Package":"x/a"}`+"\n"+`{"Action":"output","Output":"ok  \tx/a\t0.1s\n"}`+"\n")
}
