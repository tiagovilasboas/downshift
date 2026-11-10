// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"fmt"
	"strings"
	"testing"
)

// TestRun_PytestVitestExecution validates the execwrap execution, compaction on clean exit 0,
// and byte-for-byte raw preservation on exit != 0 or errors for pytest and vitest.
func TestRun_PytestVitestExecution(t *testing.T) {
	pytestClean := "============================= test session starts ==============================\nrootdir: /tmp\ncollected 25 items\n\ntest_a.py .....\ntest_b.py .....\ntest_c.py .....\ntest_d.py .....\ntest_e.py .....\n\n============================== 25 passed in 0.42s ==============================\n"
	pytestFail := "============================= test session starts ==============================\nrootdir: /tmp\ncollected 25 items\n\ntest_a.py ..F..\n\n=================================== FAILURES ===================================\n_________________________________ test_charge __________________________________\nAssertionError\n========================= 1 failed, 24 passed in 0.48s =========================\n"

	vitestClean := "\n RUN  v1.6.0 /Users/tiago.boas/web\n\n ✓ test/a.test.ts (5)\n ✓ test/b.test.ts (5)\n ✓ test/c.test.ts (5)\n ✓ test/d.test.ts (5)\n ✓ test/e.test.ts (5)\n\n Test Files  5 passed (5)\n      Tests  25 passed (25)\n   Start at  19:20:00\n   Duration  412ms\n"
	vitestFail := "\n RUN  v1.6.0 /Users/tiago.boas/web\n\n ❯ test/b.test.ts (5)\n   × test_charge\n\n⎯⎯⎯⎯⎯⎯⎯ Failed Tests 1 ⎯⎯⎯⎯⎯⎯⎯\n\n FAIL  test/b.test.ts > test_charge\nAssertionError: expected 100 to be 110\n\n Test Files  1 failed | 4 passed (5)\n      Tests  1 failed | 24 passed (25)\n"

	cases := []struct {
		name       string
		bin        string
		script     string
		argv       []string
		compact    bool
		wantCode   int
		wantClass  string
		wantOut    string
		wantErr    string
		compacted  bool
		wantFamily Family
	}{
		{
			name: "passing pytest compacts", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestClean, "\n", "\\n") + "'",
			argv:   []string{"pytest"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(pytestClean), compacted: true, wantFamily: Family("pytest"),
		},
		{
			name: "pytest kill switch keeps raw", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestClean, "\n", "\\n") + "'",
			argv:   []string{"pytest"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: pytestClean, wantFamily: Family("pytest"),
		},
		{
			name: "failing pytest stays raw", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestFail, "\n", "\\n") + "'; exit 1",
			argv:   []string{"pytest"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: pytestFail, wantFamily: Family("pytest"),
		},
		{
			name: "clean pytest with stderr compacts stdout and keeps stderr whole", bin: "pytest",
			script: "printf '" + strings.ReplaceAll(pytestClean, "\n", "\\n") + "'; echo 'pytest warning' >&2",
			argv:   []string{"pytest"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(pytestClean), compacted: true, wantErr: "pytest warning\n", wantFamily: Family("pytest"),
		},
		{
			name: "passing vitest compacts", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestClean, "\n", "\\n") + "'",
			argv:   []string{"vitest", "run"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(vitestClean), compacted: true, wantFamily: Family("vitest"),
		},
		{
			name: "vitest kill switch keeps raw", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestClean, "\n", "\\n") + "'",
			argv:   []string{"vitest", "run"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: vitestClean, wantFamily: Family("vitest"),
		},
		{
			name: "failing vitest stays raw", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestFail, "\n", "\\n") + "'; exit 1",
			argv:   []string{"vitest", "run"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: vitestFail, wantFamily: Family("vitest"),
		},
		{
			name: "clean vitest with a stderr warning compacts and keeps the warning", bin: "vitest",
			script: "printf '" + strings.ReplaceAll(vitestClean, "\n", "\\n") + "'; echo 'vitest warning' >&2",
			argv:   []string{"vitest", "run"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantFamily: Family("vitest"), compacted: true,
			wantOut: "Vitest: PASS (25 passed across 5 files in 412ms)\nvitest warning\n" +
				Marker(len(vitestClean)+len("vitest warning\n"), len("Vitest: PASS (25 passed across 5 files in 412ms)\nvitest warning\n")),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.bin, tc.script)
			r, out, errOut := run(t, tc.argv, tc.compact)
			if r.ExitCode != tc.wantCode || r.ExitClass != tc.wantClass {
				t.Fatalf("exit = %d/%s, want %d/%s (stderr %q)", r.ExitCode, r.ExitClass, tc.wantCode, tc.wantClass, errOut)
			}
			if out != tc.wantOut {
				t.Fatalf("stdout = %q, want %q", out, tc.wantOut)
			}
			if errOut != tc.wantErr {
				t.Fatalf("stderr = %q, want %q", errOut, tc.wantErr)
			}
			if r.Compacted != tc.compacted || r.Family != tc.wantFamily {
				t.Fatalf("compacted=%v family=%q, want %v %q", r.Compacted, r.Family, tc.compacted, tc.wantFamily)
			}
			if r.EmittedBytes != int64(len(out)+len(errOut)) {
				t.Fatalf("EmittedBytes = %d, want %d", r.EmittedBytes, len(out)+len(errOut))
			}
			if !r.Compacted && r.RawBytes != r.EmittedBytes {
				t.Fatalf("raw %d != emitted %d without compaction", r.RawBytes, r.EmittedBytes)
			}
		})
	}
}

// TestRun_PlaywrightCargoTscExecution validates the execwrap execution, compaction on clean exit 0,
// and byte-for-byte raw preservation on exit != 0 or errors for playwright, cargo test, and tsc.
func TestRun_PlaywrightCargoTscExecution(t *testing.T) {
	playwrightClean := "Running 25 tests using 4 workers\n\n  ✓  1 [chromium] › e2e/auth.spec.ts:10:5 › login flow (1.2s)\n  ✓  2 [chromium] › e2e/auth.spec.ts:25:5 › logout flow (850ms)\n  ✓  3 [chromium] › e2e/billing.spec.ts:12:5 › invoice generation (1.5s)\n  ✓  4 [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout (2.1s)\n  ✓  5 [chromium] › e2e/dashboard.spec.ts:8:5 › load metrics (920ms)\n\n  25 passed (4.2s)\n"
	playwrightFail := "Running 25 tests using 4 workers\n\n  ✓  1 [chromium] › e2e/auth.spec.ts:10:5 › login flow (1.2s)\n  ✘  2 [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout (2.1s)\n\n  1) [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout ───────────────────────────────────────────\n\n    Error: Timed out 5000ms waiting for expect(locator).toBeVisible()\n\n    Locator: getByText('Payment Successful')\n    Expected: visible\n    Received: <element(s) not found>\n\n      32 |     await page.getByRole('button', { name: 'Pay' }).click();\n    > 33 |     await expect(page.getByText('Payment Successful')).toBeVisible();\n         |                                                        ^\n      34 |   });\n\n        at /Users/tiago.boas/web/e2e/billing.spec.ts:33:56\n\n  1 failed\n    [chromium] › e2e/billing.spec.ts:30:5 › stripe checkout ────────────────────────────────────────────\n  24 passed (6.5s)\n"

	cargoClean := "   Compiling downshift-core v0.1.0 (/Users/tiago.boas/rust/downshift)\n    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.85s\n     Running unittests src/lib.rs (target/debug/deps/downshift_core-a1b2c3d4)\n\nrunning 25 tests\ntest auth::tests::test_token_validation ... ok\ntest auth::tests::test_session_expiry ... ok\ntest billing::tests::test_invoice_calc ... ok\ntest billing::tests::test_tax_rate ... ok\ntest config::tests::test_load_defaults ... ok\ntest store::tests::test_cache_hit ... ok\ntest store::tests::test_cache_miss ... ok\ntest utils::tests::test_sanitize ... ok\ntest utils::tests::test_slugify ... ok\ntest engine::tests::test_parse_input ... ok\n\ntest result: ok. 25 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.14s\n"
	cargoFail := "   Compiling downshift-core v0.1.0 (/Users/tiago.boas/rust/downshift)\n    Finished `test` profile [unoptimized + debuginfo] target(s) in 0.92s\n     Running unittests src/lib.rs (target/debug/deps/downshift_core-a1b2c3d4)\n\nrunning 25 tests\ntest auth::tests::test_token_validation ... ok\ntest billing::tests::test_invoice_calc ... FAILED\ntest config::tests::test_load_defaults ... ok\ntest store::tests::test_cache_hit ... ok\n\nfailures:\n\n---- billing::tests::test_invoice_calc stdout ----\nthread 'billing::tests::test_invoice_calc' panicked at src/billing.rs:42:9:\nassertion `left == right` failed\n  left: 100\n right: 110\nnote: run with `RUST_BACKTRACE=1` environment variable to display a backtrace\n\nfailures:\n    billing::tests::test_invoice_calc\n\ntest result: FAILED. 24 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.15s\n\nerror: test failed, to rerun pass `--lib`\n"

	tscClean := "[19:20:00] Projects in this build: \n    * tsconfig.json\n\n[19:20:01] Building project '/Users/tiago.boas/project/tsconfig.json'...\n[19:20:02] Found 0 errors. Watching for file changes.\n"
	tscFail := "src/components/Button.tsx:12:7 - error TS2322: Type 'string' is not assignable to type 'number'.\n\n12 const width: number = \"100px\";\n         ~~~~~\n\nsrc/utils/format.ts:45:10 - error TS2304: Cannot find name 'unknownVar'.\n\n45   return unknownVar;\n            ~~~~~~~~~~\n\nFound 2 errors in 2 files.\n\nErrors  Files\n     1  src/components/Button.tsx:12\n     1  src/utils/format.ts:45\n"

	catScript := func(body string, code int, stderr string) string {
		s := "cat << 'EOF'\n" + body + "EOF\n"
		if stderr != "" {
			s += "echo '" + stderr + "' >&2\n"
		}
		if code != 0 {
			s += fmt.Sprintf("exit %d\n", code)
		}
		return s
	}

	cases := []struct {
		name       string
		bin        string
		script     string
		argv       []string
		compact    bool
		wantCode   int
		wantClass  string
		wantOut    string
		wantErr    string
		compacted  bool
		wantFamily Family
	}{
		{
			name: "passing playwright compacts", bin: "playwright",
			script: catScript(playwrightClean, 0, ""),
			argv:   []string{"playwright", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(playwrightClean), compacted: true, wantFamily: Family("playwright"),
		},
		{
			name: "playwright kill switch keeps raw", bin: "playwright",
			script: catScript(playwrightClean, 0, ""),
			argv:   []string{"playwright", "test"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: playwrightClean, wantFamily: Family("playwright"),
		},
		{
			name: "failing playwright stays raw", bin: "playwright",
			script: catScript(playwrightFail, 1, ""),
			argv:   []string{"playwright", "test"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: playwrightFail, wantFamily: Family("playwright"),
		},
		{
			name: "clean playwright with stderr compacts stdout and keeps stderr whole", bin: "playwright",
			script: catScript(playwrightClean, 0, "playwright warning"),
			argv:   []string{"playwright", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(playwrightClean), compacted: true, wantErr: "playwright warning\n", wantFamily: Family("playwright"),
		},
		{
			name: "passing cargo test compacts", bin: "cargo",
			script: catScript(cargoClean, 0, ""),
			argv:   []string{"cargo", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(cargoClean), compacted: true, wantFamily: Family("cargo-test"),
		},
		{
			name: "cargo test kill switch keeps raw", bin: "cargo",
			script: catScript(cargoClean, 0, ""),
			argv:   []string{"cargo", "test"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: cargoClean, wantFamily: Family("cargo-test"),
		},
		{
			name: "failing cargo test stays raw", bin: "cargo",
			script: catScript(cargoFail, 1, ""),
			argv:   []string{"cargo", "test"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: cargoFail, wantFamily: Family("cargo-test"),
		},
		{
			name: "clean cargo test with stderr stays raw", bin: "cargo",
			script: catScript(cargoClean, 0, "cargo warning"),
			argv:   []string{"cargo", "test"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: cargoClean, wantErr: "cargo warning\n", wantFamily: Family("cargo-test"),
		},
		{
			name: "passing tsc compacts", bin: "tsc",
			script: catScript(tscClean, 0, ""),
			argv:   []string{"tsc"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(tscClean), compacted: true, wantFamily: Family("tsc"),
		},
		{
			name: "tsc kill switch keeps raw", bin: "tsc",
			script: catScript(tscClean, 0, ""),
			argv:   []string{"tsc"}, compact: false,
			wantCode: 0, wantClass: ExitClassZero, wantOut: tscClean, wantFamily: Family("tsc"),
		},
		{
			name: "failing tsc stays raw", bin: "tsc",
			script: catScript(tscFail, 1, ""),
			argv:   []string{"tsc"}, compact: true,
			wantCode: 1, wantClass: ExitClassNonZero, wantOut: tscFail, wantFamily: Family("tsc"),
		},
		{
			name: "clean tsc with stderr compacts stdout and keeps stderr whole", bin: "tsc",
			script: catScript(tscClean, 0, "tsc warning"),
			argv:   []string{"tsc"}, compact: true,
			wantCode: 0, wantClass: ExitClassZero, wantOut: compactedWant(tscClean), compacted: true, wantErr: "tsc warning\n", wantFamily: Family("tsc"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fakeBin(t, tc.bin, tc.script)
			r, out, errOut := run(t, tc.argv, tc.compact)
			if r.ExitCode != tc.wantCode || r.ExitClass != tc.wantClass {
				t.Fatalf("exit = %d/%s, want %d/%s (stderr %q)", r.ExitCode, r.ExitClass, tc.wantCode, tc.wantClass, errOut)
			}
			if out != tc.wantOut {
				t.Fatalf("stdout = %q, want %q", out, tc.wantOut)
			}
			if errOut != tc.wantErr {
				t.Fatalf("stderr = %q, want %q", errOut, tc.wantErr)
			}
			if r.Compacted != tc.compacted || r.Family != tc.wantFamily {
				t.Fatalf("compacted=%v family=%q, want %v %q", r.Compacted, r.Family, tc.compacted, tc.wantFamily)
			}
			if r.EmittedBytes != int64(len(out)+len(errOut)) {
				t.Fatalf("EmittedBytes = %d, want %d", r.EmittedBytes, len(out)+len(errOut))
			}
			if !r.Compacted && r.RawBytes != r.EmittedBytes {
				t.Fatalf("raw %d != emitted %d without compaction", r.RawBytes, r.EmittedBytes)
			}
		})
	}
}
