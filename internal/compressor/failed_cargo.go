// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"bytes"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Cargo test failed runs. The output is read as blocks, one per test binary:
//
//	     Running unittests src/lib.rs (target/debug/deps/x-abc)
//	<blank>
//	running N tests
//	test a ... ok
//	test b ... FAILED
//	<blank>
//	failures: ... (panic output, failure list)
//	test result: FAILED. 1 passed; 1 failed; ...
//	<blank>
//
// A crate that passed has its whole block cut. In a crate that failed only
// the "test x ... ok" lines are cut; the panic output, the "failures:" list
// and the summary counts stay. A block is cut only when it fits this grammar
// and its counts agree with its own "test result:" line (ok lines equal
// "passed", FAILED lines equal "failed"); any other block is kept byte for
// byte, and a run without "test result: FAILED." stays raw. Everything outside
// the blocks (Compiling, warnings, "error: test failed") is kept as written.

var (
	reCargoFailedSignal = regexp.MustCompile(`(?m)^test result: FAILED\. `)
	reCargoBlockStart   = regexp.MustCompile(`^\s+(?:Running \S.*|Doc-tests \S+)$`)
	reCargoRunningN     = regexp.MustCompile(`^running (\d+) tests?$`)
	reCargoTestLine     = regexp.MustCompile(`^test (\S.*) \.\.\. (ok|FAILED|ignored(?:, .*)?)$`)
	reCargoResultLine   = regexp.MustCompile(`^test result: (ok|FAILED)\. (\d+) passed; (\d+) failed; (\d+) ignored; \d+ measured; \d+ filtered out; finished in \S+$`)
)

// cargoCut counts what a failed run dropped.
type cargoCut struct{ crates, lines int }

// cargoResult is the parsed "test result:" line that closes a block.
type cargoResult struct {
	status                  string
	passed, failed, ignored int
}

// plainFailedCargoText accepts cargo test output only as it was written: no
// CRLF, escape sequences or missing final newline, and a failed test summary.
func plainFailedCargoText(input []byte) ([]byte, bool) {
	if input[len(input)-1] != '\n' || bytes.IndexByte(input, '\r') != -1 ||
		bytes.IndexByte(input, 0x1b) != -1 || !reCargoFailedSignal.Match(input) {
		return nil, false
	}
	return input, true
}

// compactFailedCargo cuts the passing noise of a failed cargo test run and
// prepends one summary line. The reason is non-empty when the run stays raw.
func compactFailedCargo(text string) (string, string) {
	lines := strings.Split(strings.TrimSuffix(text, "\n"), "\n")
	var kept []string
	var cut cargoCut
	for i := 0; i < len(lines); {
		if !reCargoBlockStart.MatchString(lines[i]) {
			kept = append(kept, lines[i])
			i++
			continue
		}
		end := cargoBlockEnd(lines, i)
		out, crate, n := cutCargoBlock(lines[i:end])
		kept = append(kept, out...)
		cut.lines += n
		if crate {
			cut.crates++
		}
		i = end
	}
	if cut.lines == 0 && cut.crates == 0 {
		return "", "cargo_test_failed_run_no_gain"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Cargo test: %d passing crates, %d passing test lines cut (failures kept verbatim)\n", cut.crates, cut.lines)
	for _, l := range kept {
		b.WriteString(l + "\n")
	}
	return b.String(), ""
}

// cargoBlockEnd returns the index after the block that starts at start: up to
// and including its "test result:" line and the blank line after it. A block
// that meets another start or the end of input first stops there unchanged.
func cargoBlockEnd(lines []string, start int) int {
	for j := start + 1; j < len(lines); j++ {
		if reCargoBlockStart.MatchString(lines[j]) {
			return j
		}
		if strings.HasPrefix(lines[j], "test result: ") {
			if j+1 < len(lines) && lines[j+1] == "" {
				return j + 2
			}
			return j + 1
		}
	}
	return len(lines)
}

// cutCargoBlock returns the lines of one block to keep, whether the whole
// crate was cut, and how many test lines were cut. A block outside the
// grammar comes back unchanged.
func cutCargoBlock(block []string) (kept []string, crate bool, cutLines int) {
	if len(block) < 4 || block[1] != "" {
		return block, false, 0
	}
	m := reCargoRunningN.FindStringSubmatch(block[2])
	if m == nil {
		return block, false, 0
	}
	want, _ := strconv.Atoi(m[1])
	k := 3
	var ok, failed, ignored int
	for ; k < len(block); k++ {
		t := reCargoTestLine.FindStringSubmatch(block[k])
		if t == nil {
			break
		}
		switch {
		case t[2] == "ok":
			ok++
		case t[2] == "FAILED":
			failed++
		default:
			ignored++
		}
	}
	if ok+failed+ignored != want || k >= len(block) || block[k] != "" {
		return block, false, 0
	}
	rest := block[k+1:]
	res := cargoResultOf(rest)
	if res == nil || res.passed != ok || res.failed != failed || res.ignored != ignored {
		return block, false, 0
	}
	if res.status == "ok" {
		if len(rest) > 2 {
			return block, false, 0
		}
		return nil, true, ok
	}
	kept = append(kept, block[:3]...)
	for _, l := range block[3:k] {
		if !strings.HasSuffix(l, " ... ok") {
			kept = append(kept, l)
		}
	}
	kept = append(kept, block[k])
	return append(kept, rest...), false, ok
}

// cargoResultOf reads the "test result:" line that closes the rest of a
// block: the last line, ignoring a trailing blank line.
func cargoResultOf(rest []string) *cargoResult {
	for len(rest) > 0 && rest[len(rest)-1] == "" {
		rest = rest[:len(rest)-1]
	}
	if len(rest) == 0 {
		return nil
	}
	m := reCargoResultLine.FindStringSubmatch(rest[len(rest)-1])
	if m == nil {
		return nil
	}
	p, _ := strconv.Atoi(m[2])
	f, _ := strconv.Atoi(m[3])
	i, _ := strconv.Atoi(m[4])
	return &cargoResult{status: m[1], passed: p, failed: f, ignored: i}
}
