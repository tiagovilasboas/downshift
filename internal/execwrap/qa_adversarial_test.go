// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package execwrap

import (
	"bytes"
	"os"
	"strings"
	"sync"
	"testing"
)

// These tests pin real defects found by QA of `downshift exec`. They follow
// the convention of internal/compressor/corpus_test.go: each is skipped unless
// DOWNSHIFT_CORPUS_STRICT=1, so the default suite stays green while the
// defect is open. When a defect is fixed, drop its skip guard.
func qaStrict(t *testing.T) {
	t.Helper()
	if os.Getenv("DOWNSHIFT_CORPUS_STRICT") != "1" {
		t.Skip("open defect; run with DOWNSHIFT_CORPUS_STRICT=1")
	}
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

// An all-ok `go test -cover` run is the shape the compressor summarises. The
// summary drops the coverage figures the user asked for.
func TestQA_CoverageFiguresSurviveCompaction(t *testing.T) {
	qaStrict(t)
	var sb strings.Builder
	for _, p := range []string{"a", "b", "c", "d", "e", "f"} {
		sb.WriteString("ok  \tgithub.com/x/" + p + "\t0.1s\tcoverage: 65.9% of statements\n")
	}
	fakeBin(t, "go", "printf '%s' '"+sb.String()+"'")
	_, out, _ := run(t, []string{"go", "test", "-cover", "./..."}, true)
	if !strings.Contains(out, "65.9%") {
		t.Fatalf("coverage figures lost: %q", out)
	}
}

// Output that does not end in a newline may be cut off; it must stay raw.
func TestQA_OutputWithoutFinalNewlineStaysRaw(t *testing.T) {
	qaStrict(t)
	fakeBin(t, "go", `printf 'ok  \tgithub.com/x/a\t0.1s\nok  \tgithub.com/x/b\t0.1s\nok  \tgithub.com/x/c\t0.1s\nok  \tgithub.com/x/d\t0.1'`)
	_, out, _ := run(t, []string{"go", "test", "./..."}, true)
	if !strings.Contains(out, "github.com/x/d") {
		t.Fatalf("possibly truncated output was summarised: %q", out)
	}
}

// A caller that merges stdout and stderr (the usual shell tool) must see them
// in the order the program wrote them. Holding stdout until exit reorders them.
func TestQA_StdoutStderrOrderPreserved(t *testing.T) {
	qaStrict(t)
	fakeBin(t, "go", `echo out1; sleep 0.1; echo err1 >&2; sleep 0.1; echo out2`)
	var merged lockedBuf
	Run([]string{"go", "test", "./..."}, Options{Stdin: strings.NewReader(""), Stdout: &merged, Stderr: &merged, Compact: true})
	if got := merged.b.String(); got != "out1\nerr1\nout2\n" {
		t.Fatalf("interleaving changed: %q", got)
	}
}
