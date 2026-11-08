// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// devcheck is the repository gate behind `make check` (-changed: only what the
// change touches, see changed.go), `make check-full` and `make check-fast`.
// It runs the static stages (gofmt and go vet for host, windows and linux)
// concurrently, prints each stage's output in the fixed order the Makefile used,
// and then, unless -fast is set, runs go test -count=1 ./... with output
// streamed. Any failure exits 1; make turns that into its own exit 2, as before.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift-ai
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	fast := flag.Bool("fast", false, "static stages only (gofmt, go vet); no tests")
	changed := flag.Bool("changed", false, "gofmt, vet and tests of the packages changed since origin/main only (make check)")
	flag.Parse()
	if *changed {
		os.Exit(runChanged(os.Stdout))
	}
	os.Exit(run(*fast))
}

// run returns the process exit code: 0 when every stage passed, 1 otherwise.
func run(fast bool) int {
	if !runStatic(os.Stdout, staticStages()) {
		return 1
	}
	if fast {
		return 0
	}
	// The Makefile numbered the static stages [1/4]..[4/4] and the tests [5/5];
	// the headers are kept as they were so logs and habits do not change.
	fmt.Println("[5/5] go test -count=1 ./...")
	if err := runTests(os.Stdout, os.Stderr); err != nil {
		return 1
	}
	return 0
}
