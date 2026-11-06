// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestFormatResultCleanTreePasses(t *testing.T) {
	out, ok := formatResult(nil, nil)
	if !ok || len(out) != 0 {
		t.Fatalf("clean tree: out = %q, ok = %v", out, ok)
	}
}

func TestFormatResultListsFilesAndTheHint(t *testing.T) {
	out, ok := formatResult([]byte("a.go\nb.go\n"), nil)
	want := "a.go\nb.go\n" + formatHint
	if ok || string(out) != want {
		t.Fatalf("out = %q, ok = %v, want %q and a failure", out, ok, want)
	}
}

func TestFormatResultFailsWhenGofmtErrors(t *testing.T) {
	out, ok := formatResult(nil, errors.New("exit status 2"))
	if ok || !bytes.Contains(out, []byte("gofmt: exit status 2")) {
		t.Fatalf("out = %q, ok = %v", out, ok)
	}
}

// Stages run at once but print in list order, and a failure in one stage does
// not stop the others from reporting.
func TestRunStaticPrintsInOrderAndReportsEveryStage(t *testing.T) {
	slow := func() ([]byte, bool) { time.Sleep(20 * time.Millisecond); return []byte("first out\n"), false }
	fast := func() ([]byte, bool) { return []byte("second out\n"), true }
	var buf bytes.Buffer
	ok := runStatic(&buf, []stage{{"[1/2] slow", slow}, {"[2/2] fast", fast}})
	want := "[1/2] slow\nfirst out\n[2/2] fast\nsecond out\n"
	if ok || buf.String() != want {
		t.Fatalf("ok = %v, output = %q, want %q and a failure", ok, buf.String(), want)
	}
}

func TestRunStaticAllPass(t *testing.T) {
	pass := func() ([]byte, bool) { return nil, true }
	var buf bytes.Buffer
	if !runStatic(&buf, []stage{{"[1/1] x", pass}}) || buf.String() != "[1/1] x\n" {
		t.Fatalf("output = %q", buf.String())
	}
}
