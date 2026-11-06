// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVersionFromOutputAcceptsOnlyAVersion(t *testing.T) {
	cases := map[string]string{
		"downshift v0.8.1 (commit abc, built x)": "v0.8.1",
		"downshift dev":                          "dev",
		"downshift $(rm -rf ~) now":              "",
		"downshift \x1b[31mred":                  "",
		"other v0.8.1":                           "",
		"":                                       "",
	}
	for in, want := range cases {
		if got := versionFromOutput(in); got != want {
			t.Errorf("versionFromOutput(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNewestOnPathRunsAtMostTheCap(t *testing.T) {
	root := t.TempDir()
	var copies []string
	for _, d := range []string{"a", "b", "c", "d", "e", "f"} {
		copies = append(copies, filepath.Join(root, d, "downshift"))
	}
	ran := 0
	run := func(string) (string, error) { ran++; return "downshift v0.8.1", nil }
	env := doctorEnv{Home: t.TempDir(), RunVersion: run, PathCopies: func() []string { return copies }}
	checkVersionSync(&doctorReport{}, env, "v0.8.1")
	if ran != maxPathCopiesRun {
		t.Fatalf("ran %d copies, want %d", ran, maxPathCopiesRun)
	}
}

func TestStampSyncSkipsPseudoVersionBinary(t *testing.T) {
	home := t.TempDir()
	writePluginStamp(t, home, "v0.8.1")
	rep := &doctorReport{}
	checkVersionSync(rep, doctorEnv{Home: home}, "v0.8.2-0.20261106032522-b906e1316e3e")
	if row, ok := versionRow(rep, versionSyncID); ok && row.Status == doctorFail {
		t.Fatalf("a pseudo-version build must not fail the plugin stamp: %+v", row)
	}
}

func TestLatestCacheFromTheFutureIsNotFresh(t *testing.T) {
	if fresh(-time.Hour) {
		t.Error("a negative age must not be fresh")
	}
	if !fresh(time.Hour) || fresh(25*time.Hour) {
		t.Error("fresh must hold inside the TTL only")
	}
}

func TestCircuitLineSaysTheRunLength(t *testing.T) {
	c := routingCircuit{Harness: "opencode", Code: "INVALID_REQUEST", Failures: 7}
	if !strings.Contains(c.line(), "after 7 INVALID_REQUEST") {
		t.Errorf("line = %q", c.line())
	}
	c.Failures = 0
	if !strings.Contains(c.line(), "after 3 INVALID_REQUEST") {
		t.Errorf("line with no count = %q", c.line())
	}
}
