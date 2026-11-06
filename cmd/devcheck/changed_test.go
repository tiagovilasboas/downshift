// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainSelection(t *testing.T) {
	cases := []struct {
		name  string
		files []string
		pkgs  []string
		want  string
	}{
		{"narrow", []string{"internal/a/a.go"}, []string{"x/internal/a", "x/cmd/b"}, "1 changed file(s), 2 package(s) with dependents"},
		{"go.mod", []string{"go.mod", "internal/a/a.go"}, []string{"./..."}, "all packages: cannot narrow go.mod"},
		{"root file", []string{"Makefile", "README.md"}, []string{"./..."}, "all packages: cannot narrow Makefile"},
		{"no package", []string{"internal/a/a.go"}, []string{"./..."}, "all packages: a changed file has no Go package above it"},
	}
	for _, c := range cases {
		if got := explainSelection(c.files, c.pkgs); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestSummarizeTestsCleanRun(t *testing.T) {
	in := `{"Action":"pass","Package":"p","Test":"TestA"}` + "\n" + `{"Action":"pass","Package":"p"}` + "\n"
	if report, ok := summarizeTests(strings.NewReader(in)); !ok || report != "" {
		t.Fatalf("report = %q, ok = %v", report, ok)
	}
}

func TestSummarizeTestsShowsOnlyFailuresAndRerun(t *testing.T) {
	in := `{"Action":"output","Package":"p","Test":"TestGood","Output":"noise\n"}
{"Action":"pass","Package":"p","Test":"TestGood"}
{"Action":"output","Package":"p","Test":"TestBad","Output":"bad.go:3: boom\n"}
{"Action":"fail","Package":"p","Test":"TestBad"}
{"Action":"fail","Package":"p"}
`
	report, ok := summarizeTests(strings.NewReader(in))
	want := "--- FAIL p TestBad\nbad.go:3: boom\nrerun: go test -count=1 p\n"
	if ok || report != want {
		t.Fatalf("report = %q, ok = %v, want %q", report, ok, want)
	}
}

func TestSummarizeTestsBuildFailure(t *testing.T) {
	in := `{"ImportPath":"p","Action":"build-output","Output":"p.go:1: undefined: x\n"}
{"Action":"fail","Package":"p"}
`
	report, ok := summarizeTests(strings.NewReader(in))
	if ok || !strings.Contains(report, "undefined: x") || !strings.Contains(report, "rerun: go test -count=1 p") {
		t.Fatalf("report = %q, ok = %v", report, ok)
	}
}

func TestTestParallelismEnvOverrideAndBounds(t *testing.T) {
	t.Setenv("DOWNSHIFT_HOOK_TEST_P", "3")
	if got := testParallelism(); got != 3 {
		t.Fatalf("override: got %d", got)
	}
	t.Setenv("DOWNSHIFT_HOOK_TEST_P", "")
	if got := testParallelism(); got < 2 || got > 6 {
		t.Fatalf("default out of 2..6: %d", got)
	}
}

func TestGreenStamp(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "x.green")
	if isGreen(path, "k1") {
		t.Fatal("no stamp yet, must not be green")
	}
	recordGreen(path, "k1")
	if !isGreen(path, "k1") {
		t.Fatal("recorded key must be green")
	}
	if isGreen(path, "k2") {
		t.Fatal("a different key must not be green")
	}
}
