// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

// testParallelism is the -p for go test: half the CPUs, between 2 and 6, so
// parallel worktrees share the machine. DOWNSHIFT_HOOK_TEST_P overrides it.
func testParallelism() int {
	if n, err := strconv.Atoi(os.Getenv("DOWNSHIFT_HOOK_TEST_P")); err == nil && n > 0 {
		return n
	}
	return min(max(runtime.NumCPU()/2, 2), 6)
}

type testEvent struct {
	Action     string
	Package    string
	ImportPath string
	Test       string
	Output     string
}

// summarizeTests reads `go test -json` and returns what failed: each failing
// test with its own output, the package output when a package failed without a
// failing test (a build error), and the exact command to rerun each failed
// package. ok is true when nothing failed.
func summarizeTests(r io.Reader) (report string, ok bool) {
	outputs := map[string][]string{} // package + "\x00" + test
	var failedTests, failedPkgs []string
	hasFailedTest := map[string]bool{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for sc.Scan() {
		var ev testEvent
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue
		}
		if ev.Action == "build-output" { // compiler errors, keyed by import path
			ev.Package, ev.Action = ev.ImportPath, "output"
		}
		if ev.Package == "" {
			continue
		}
		key := ev.Package + "\x00" + ev.Test
		switch ev.Action {
		case "output":
			outputs[key] = append(outputs[key], ev.Output)
		case "fail":
			if ev.Test != "" {
				failedTests = append(failedTests, key)
				hasFailedTest[ev.Package] = true
			} else {
				failedPkgs = append(failedPkgs, ev.Package)
			}
		}
	}
	if len(failedTests) == 0 && len(failedPkgs) == 0 {
		return "", true
	}
	var b strings.Builder
	for _, key := range failedTests {
		pkg, test, _ := strings.Cut(key, "\x00")
		fmt.Fprintf(&b, "--- FAIL %s %s\n%s", pkg, test, strings.Join(outputs[key], ""))
	}
	for _, pkg := range failedPkgs {
		if !hasFailedTest[pkg] {
			fmt.Fprintf(&b, "--- FAIL %s\n%s", pkg, strings.Join(outputs[pkg+"\x00"], ""))
		}
	}
	for _, pkg := range failedPkgs {
		fmt.Fprintf(&b, "rerun: go test -count=1 %s\n", pkg)
	}
	return b.String(), false
}
