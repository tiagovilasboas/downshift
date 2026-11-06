// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// outcome is one finished stage of the changed-packages gate.
type outcome struct {
	name   string
	took   time.Duration
	ok     bool
	detail string // shown only on failure
}

// runChanged is `make check`: gofmt, the three vets and the tests of the
// packages changed since origin/main (and their dependents) all run at once,
// tests through the Go test cache. A tree identical to the last green run
// returns at once. It returns the process exit code.
func runChanged(w io.Writer) int {
	start := time.Now()
	base, err := mergeBase()
	if err != nil {
		fmt.Fprintln(w, "check:", err)
		return 1
	}
	files, err := changedFiles(base)
	if err != nil {
		fmt.Fprintln(w, "check:", err)
		return 1
	}
	if len(files) == 0 {
		fmt.Fprintln(w, "check: no changes since origin/main")
		return 0
	}
	stamp, key := greenKey(base)
	if stamp != "" && isGreen(stamp, key) {
		fmt.Fprintln(w, "check: unchanged since last green run")
		return 0
	}
	pkgs, err := selectPackages(files)
	if err != nil {
		fmt.Fprintln(w, "check:", err)
		return 1
	}
	fmt.Fprintf(w, "check: %s: %s\n", describe(pkgs), explainSelection(files, pkgs))

	results := runStages(w, pkgs)
	failed := 0
	for _, r := range results {
		if !r.ok {
			failed++
			fmt.Fprintf(w, "\n%s failed:\n%s", r.name, r.detail)
		}
	}
	elapsed := time.Since(start).Round(100 * time.Millisecond)
	if failed > 0 {
		fmt.Fprintf(w, "check: FAILED in %s (%d of %d stages)\n", elapsed, failed, len(results))
		return 1
	}
	if stamp != "" {
		recordGreen(stamp, key)
	}
	fmt.Fprintf(w, "check: OK in %s (%s)\n", elapsed, describe(pkgs))
	return 0
}

// greenKey returns the stamp path and the tree key; both empty when they cannot be computed.
func greenKey(base string) (string, string) {
	untracked, err := gitOut("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return "", ""
	}
	key, err := treeKey(base, strings.Fields(untracked))
	path, perr := stampPath()
	if err != nil || perr != nil {
		return "", ""
	}
	return path, key
}

func describe(pkgs []string) string {
	if len(pkgs) == 1 && pkgs[0] == "./..." {
		return "all pkgs"
	}
	return fmt.Sprintf("%d pkgs", len(pkgs))
}

// runStages starts every stage at once and prints one line per stage as it ends.
func runStages(w io.Writer, pkgs []string) []outcome {
	stages := []struct {
		name string
		run  func() (string, bool)
	}{
		{"gofmt", func() (string, bool) { o, ok := checkFormat(); return string(o), ok }},
		{"vet host", vetPkgs(nil, pkgs)},
		{"vet windows", vetPkgs([]string{"GOOS=windows", "GOARCH=amd64"}, pkgs)},
		{"vet linux", vetPkgs([]string{"GOOS=linux", "GOARCH=amd64"}, pkgs)},
		{"test", func() (string, bool) { return testPkgs(pkgs) }},
	}
	results := make([]outcome, len(stages))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i, s := range stages {
		wg.Go(func() {
			t := time.Now()
			detail, ok := s.run()
			r := outcome{s.name, time.Since(t), ok, detail}
			results[i] = r
			mu.Lock()
			defer mu.Unlock()
			status := "ok  "
			if !ok {
				status = "FAIL"
			}
			fmt.Fprintf(w, "  %s %-12s %s\n", status, r.name, r.took.Round(100*time.Millisecond))
		})
	}
	wg.Wait()
	return results
}

func vetPkgs(env []string, pkgs []string) func() (string, bool) {
	return func() (string, bool) {
		out, err := combined(env, "go", append([]string{"vet"}, pkgs...)...)
		return string(out), err == nil
	}
}

// testPkgs runs go test -json with the Go test cache on (a package whose inputs
// did not change is not run again) and reports only the failures.
func testPkgs(pkgs []string) (string, bool) {
	args := append([]string{"test", "-json", "-p", fmt.Sprint(testParallelism())}, pkgs...)
	cmd := exec.Command("go", args...)
	scale := os.Getenv("DOWNSHIFT_TEST_TIMEOUT_SCALE")
	if scale == "" {
		scale = "4" // the hooks run while other worktrees load the machine
	}
	cmd.Env = append(os.Environ(), "DOWNSHIFT_TEST_TIMEOUT_SCALE="+scale)
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	report, ok := summarizeTests(&out)
	if err != nil && ok { // go test failed without a failing test: show what it said
		report, ok = stderr.String()+out.String(), false
	}
	return report, ok && err == nil
}
