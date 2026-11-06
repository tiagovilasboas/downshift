// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package compressor

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// `go test -json` prints one JSON event per line (test2json): run, output,
// pass, fail, skip, pause, cont, with the package and test it belongs to.

// reGoJSONFirst is the first event line of a `go test -json` stream.
var reGoJSONFirst = regexp.MustCompile(`^\{(?:"Time":"[^"]*",)?"Action":"(?:start|run|output|pass|fail|skip|pause|cont|name|bench)"`)

// reGoJSONDanger marks test output that must keep a run raw even when the
// test passed: a panic, a data race, a fatal runtime error or a failed header.
var reGoJSONDanger = regexp.MustCompile(`(?i)panic:|data race|fatal error:|--- FAIL|^FAIL\b|\[(?:build|setup) failed\]`)

// goJSONEvent is one decoded event line plus the bytes it came from.
type goJSONEvent struct {
	Action, Package, Test, Output string
	Elapsed                       float64
	raw                           string
}

// isGoJSON reports whether input is a `go test -json` stream.
func isGoJSON(input []byte) bool {
	end := strings.IndexByte(string(input[:min(len(input), 512)]), '\n')
	if end < 0 {
		end = min(len(input), 512)
	}
	return reGoJSONFirst.Match(input[:end])
}

// parseGoJSON decodes every line of the stream; ok is false when any line is
// not a test2json event, so a mixed stream stays raw.
func parseGoJSON(text string) ([]goJSONEvent, bool) {
	var events []goJSONEvent
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		var e goJSONEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil || e.Action == "" {
			return nil, false
		}
		e.raw = line
		events = append(events, e)
	}
	return events, len(events) > 0
}

type goJSONPackage struct {
	name              string
	passed, skipped   int
	cover, noTestFile string
	result            string
	elapsed           float64
	kept              []string
}

// compressGoTestJSON folds a clean `go test -json` run into one line per
// package. Output of a passing test is folded; package-level output other
// than the PASS and ok lines is kept verbatim. A failed test or package, a
// danger line, an unfinished test or a line that is not an event keeps the run raw.
func compressGoTestJSON(input []byte) ([]byte, bool, string) {
	events, ok := parseGoJSON(string(input))
	if !ok {
		return input, false, "go_test_json_unparsed"
	}
	pkgs := map[string]*goJSONPackage{}
	var order []string
	running := map[string]bool{}
	for _, e := range events {
		p := pkgs[e.Package]
		if p == nil {
			p = &goJSONPackage{name: e.Package}
			pkgs[e.Package] = p
			order = append(order, e.Package)
		}
		key := e.Package + "\x00" + e.Test
		switch {
		case e.Action == "fail" || e.Action == "bench":
			return input, false, "go_test_json_failure_preserved"
		case e.Action == "run":
			running[key] = true
		case e.Action == "output" && reGoJSONDanger.MatchString(e.Output):
			return input, false, "go_test_json_danger_line_preserved"
		case e.Action == "output" && e.Test == "":
			if reason := p.packageOutput(e.Output); reason != "" {
				return input, false, reason
			}
		case (e.Action == "pass" || e.Action == "skip") && e.Test != "":
			delete(running, key)
			if !strings.Contains(e.Test, "/") {
				p.count(e.Action)
			}
		case (e.Action == "pass" || e.Action == "skip") && e.Test == "":
			p.result, p.elapsed = e.Action, e.Elapsed
		}
	}
	if len(running) > 0 {
		return input, false, "go_test_json_incomplete_transcript"
	}
	return renderGoJSON(input, pkgs, order)
}

func (p *goJSONPackage) count(action string) {
	if action == "skip" {
		p.skipped++
	} else {
		p.passed++
	}
}

// packageOutput files one package-level output line: the PASS marker and the
// ok line are re-stated by the summary, coverage and "no test files" fold into
// it, and any other line stays verbatim. It returns a reason to keep the run raw.
func (p *goJSONPackage) packageOutput(out string) string {
	line := strings.TrimRight(out, "\n")
	switch {
	case line == "PASS" || strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "ok\t") || strings.TrimSpace(line) == "":
	case strings.HasPrefix(line, "coverage: "):
		p.cover = strings.TrimSuffix(strings.TrimPrefix(line, "coverage: "), " of statements")
	case strings.HasPrefix(line, "?") && strings.HasSuffix(line, "[no test files]"):
		p.noTestFile = "yes"
	case goFoldDanger.MatchString(line):
		return "go_test_json_danger_line_preserved"
	default:
		p.kept = append(p.kept, strings.TrimRight(out, "\n"))
	}
	return ""
}

func renderGoJSON(input []byte, pkgs map[string]*goJSONPackage, order []string) ([]byte, bool, string) {
	var b strings.Builder
	passed, skipped, okPkgs := 0, 0, 0
	for _, name := range order {
		passed += pkgs[name].passed
		skipped += pkgs[name].skipped
		if pkgs[name].noTestFile == "" {
			okPkgs++
		}
	}
	if okPkgs == 0 && passed == 0 {
		return input, false, "go_test_json_no_tests"
	}
	fmt.Fprintf(&b, "Go test -json: PASS (%d packages ok, %d passed, %d skipped)\n", okPkgs, passed, skipped)
	for _, name := range order {
		p := pkgs[name]
		if p.noTestFile != "" {
			fmt.Fprintf(&b, "? %s [no test files]\n", name)
			continue
		}
		fmt.Fprintf(&b, "ok %s %.2fs (%d passed", name, p.elapsed, p.passed)
		if p.skipped > 0 {
			fmt.Fprintf(&b, ", %d skipped", p.skipped)
		}
		b.WriteString(")")
		if p.cover != "" {
			b.WriteString(" coverage " + p.cover)
		}
		b.WriteString("\n")
		for _, l := range p.kept {
			b.WriteString(l + "\n")
		}
	}
	if b.Len() >= len(input) {
		return input, false, "no_compression_gain"
	}
	return []byte(b.String()), true, fmt.Sprintf("compressed go test -json results (%d passed)", passed)
}
