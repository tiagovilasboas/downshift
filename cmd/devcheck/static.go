// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"sync"
)

// stage is one static check: a header line, as the Makefile printed it, and the
// work. check returns the stage's output and whether it passed.
type stage struct {
	header string
	check  func() (out []byte, ok bool)
}

// staticStages lists the checks in the order their output is printed.
func staticStages() []stage {
	return []stage{
		{"[1/4] gofmt -l .", checkFormat},
		{"[2/4] go vet ./...  (host)", vetFor(nil)},
		{"[3/4] go vet ./...  (GOOS=windows: type-checks test files the Windows CI job compiles)", vetFor([]string{"GOOS=windows", "GOARCH=amd64"})},
		{"[4/4] go vet ./...  (GOOS=linux)", vetFor([]string{"GOOS=linux", "GOARCH=amd64"})},
	}
}

// runStatic runs every stage at once, then prints each header and output in
// list order, so concurrent stages never interleave. It reports whether all
// stages passed. Every stage runs to the end, so one run shows all failures.
func runStatic(w io.Writer, stages []stage) bool {
	type result struct {
		out []byte
		ok  bool
	}
	results := make([]result, len(stages))
	var wg sync.WaitGroup
	for i, s := range stages {
		wg.Go(func() {
			out, ok := s.check()
			results[i] = result{out, ok}
		})
	}
	wg.Wait()

	allOK := true
	for i, s := range stages {
		_, _ = io.WriteString(w, s.header+"\n")
		_, _ = w.Write(results[i].out)
		allOK = allOK && results[i].ok
	}
	return allOK
}

// combined runs name with args and extra environment and returns its standard
// output and error together.
func combined(env []string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), env...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.Bytes(), err
}
