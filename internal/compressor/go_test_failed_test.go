// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/compressor"
)

var updateFailedGolden = flag.Bool("update-failed-golden", false, "rewrite testdata/failed_go_test/*.golden")

const failedGoTestDir = "testdata/failed_go_test"

// passingNoise matches the lines a failed run may drop.
var passingNoise = regexp.MustCompile(`^(?:ok\s|\s*--- PASS:)`)

// keepLines are the lines that must survive byte for byte and in order.
var keepLines = regexp.MustCompile(`^(?:\s*--- FAIL:|FAIL|\s*panic:|# |WARNING: DATA RACE|internal/|goroutine |\s+\S+_test\.go:\d+:|\s+testing\.go:|\s+/|exit status|=+$)|\[build failed\]|^\t`)

func TestCompressFailedGoTestGolden(t *testing.T) {
	ins, err := filepath.Glob(filepath.Join(failedGoTestDir, "*.in"))
	if err != nil || len(ins) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	for _, inPath := range ins {
		name := strings.TrimSuffix(filepath.Base(inPath), ".in")
		t.Run(name, func(t *testing.T) {
			in, err := os.ReadFile(inPath)
			if err != nil {
				t.Fatal(err)
			}
			r := compressor.CompressFailed(in, compressor.FormatGoTest)
			if !r.Applied || len(r.Output) >= len(in) {
				t.Fatalf("not compacted: applied=%v reason=%q %d -> %d", r.Applied, r.Reason, len(in), len(r.Output))
			}
			goldenPath := strings.TrimSuffix(inPath, ".in") + ".golden"
			if *updateFailedGolden {
				if err := os.WriteFile(goldenPath, r.Output, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(r.Output, golden) {
				t.Fatalf("output differs from golden\n got: %q\nwant: %q", r.Output, golden)
			}
			assertFailureLinesSurvive(t, in, r.Output)
		})
	}
}

// assertFailureLinesSurvive checks that every non-noise input line appears in
// the output byte-identical and in order, that the only extra line is the
// summary, and that no passing noise is left.
func assertFailureLinesSurvive(t *testing.T, in, out []byte) {
	t.Helper()
	outLines := strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
	if !strings.HasPrefix(outLines[0], "Go test: ") {
		t.Fatalf("missing summary line: %q", outLines[0])
	}
	outLines = outLines[1:]
	passed, failed := map[string]bool{}, map[string]bool{}
	for _, l := range strings.Split(string(in), "\n") {
		if m := regexp.MustCompile(`^\s*--- (PASS|FAIL): (\S+) `).FindStringSubmatch(l); m != nil {
			if m[1] == "PASS" {
				passed[m[2]] = true
			} else {
				failed[m[2]] = true
			}
		}
	}
	runHeader := regexp.MustCompile(`^=== (?:RUN|PAUSE|CONT|NAME)\s+(\S+)$`)
	var want []string
	for _, l := range strings.Split(strings.TrimSuffix(string(in), "\n"), "\n") {
		if passingNoise.MatchString(l) {
			continue
		}
		if m := runHeader.FindStringSubmatch(l); m != nil && passed[m[1]] && !failed[m[1]] {
			continue
		}
		want = append(want, l)
	}
	if strings.Join(want, "\n") != strings.Join(outLines, "\n") {
		t.Fatalf("non-noise lines differ\n got: %q\nwant: %q", outLines, want)
	}
}

func TestCompressFailedGoTestKeepsEveryFailureBlock(t *testing.T) {
	for _, n := range []string{"plain_one_failure", "verbose_failure", "panic", "build_error", "data_race", "timeout"} {
		in, err := os.ReadFile(filepath.Join(failedGoTestDir, n+".in"))
		if err != nil {
			t.Fatal(err)
		}
		out := compressor.CompressFailed(in, compressor.FormatGoTest).Output
		for _, l := range strings.Split(string(in), "\n") {
			if keepLines.MatchString(l) && !strings.Contains(string(out), l+"\n") {
				t.Errorf("%s: lost failure line %q", n, l)
			}
		}
	}
}

func TestCompressFailedGoTestFailsOpen(t *testing.T) {
	cases := map[string]string{
		"no failure signal":   "ok  \ta/b\t0.1s\nok  \ta/c\t0.1s\n",
		"crlf":                "ok  \ta/b\t0.1s\r\n--- FAIL: TestX (0.00s)\r\nFAIL\r\n",
		"no final newline":    "ok  \ta/b\t0.1s\n--- FAIL: TestX (0.00s)\nFAIL",
		"json events":         "{\"Action\":\"run\",\"Test\":\"TestX\"}\n{\"Action\":\"fail\",\"Test\":\"TestX\"}\nFAIL\n",
		"nothing to cut":      "--- FAIL: TestX (0.00s)\n    x_test.go:1: boom\nFAIL\nFAIL\ta/b\t0.1s\n",
		"binary":              "ok  \ta/b\t0.1s\n\x00FAIL\n",
		"empty":               "",
		"coverage ok is kept": "FAIL\ta/b\t0.1s\n",
	}
	for name, s := range cases {
		r := compressor.CompressFailed([]byte(s), compressor.FormatGoTest)
		if r.Applied || string(r.Output) != s {
			t.Errorf("%s: want raw, got applied=%v output=%q", name, r.Applied, r.Output)
		}
	}
	if r := compressor.CompressFailed([]byte("ok  \ta/b\t0.1s\nFAIL\n"), compressor.FormatPytest); r.Applied {
		t.Error("other formats must stay raw on a failed run")
	}
}

func TestCompressFailedGoTestKeepsCoverageAndLoggedTests(t *testing.T) {
	in := "ok  \ta/b\t0.1s\tcoverage: 80.0% of statements\nok  \ta/c\t0.1s\nok  \ta/d\t0.1s\n" +
		"=== RUN   TestLog\n=== RUN   TestQuiet\n--- PASS: TestQuiet (0.00s)\n--- PASS: TestLog (0.00s)\n    log_test.go:3: hello\n" +
		"--- FAIL: TestBad (0.00s)\nFAIL\n"
	_ = in
	in = strings.Replace(in, "ok  \ta/b\t0.1s\tcoverage", "ok  \ta/b\t0.1s coverage", 1)
	out := string(compressor.CompressFailed([]byte(in), compressor.FormatGoTest).Output)
	for _, must := range []string{"coverage: 80.0%", "=== RUN   TestLog\n", "--- PASS: TestLog (0.00s)\n", "    log_test.go:3: hello\n"} {
		if !strings.Contains(out, must) {
			t.Errorf("lost %q in %q", must, out)
		}
	}
	for _, gone := range []string{"TestQuiet", "ok  \ta/c"} {
		if strings.Contains(out, gone) {
			t.Errorf("kept noise %q in %q", gone, out)
		}
	}
}

func TestCompressExitStillPreservesNonzero(t *testing.T) {
	in, _ := os.ReadFile(filepath.Join(failedGoTestDir, "panic.in"))
	if r := compressor.CompressExit(in, compressor.ModeSafe, 1); r.Applied || !bytes.Equal(r.Output, in) {
		t.Fatal("CompressExit must keep preserving non-zero exits; the wrapper opts in through CompressFailed")
	}
}
