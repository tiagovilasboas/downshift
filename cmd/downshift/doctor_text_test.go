// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// Paths under home print as ~, and no line is wider than the terminal.
func TestDoctorText_HomeRelativeAndFitsWidth(t *testing.T) {
	rep := &doctorReport{home: "/Users/example", width: 60}
	rep.add(doctorCheck{ID: "state dir", Status: doctorOK,
		Detail: "/Users/example/.downshift/some/very/long/location/that/keeps/going/events.jsonl (env)"})
	rep.add(doctorCheck{ID: "legacy dir", Status: doctorWarn,
		Detail: "/Users/example/.harness-downshift is in use",
		Fix:    "cp -R /Users/example/.harness-downshift /Users/example/.downshift"})
	var b bytes.Buffer
	rep.writeText(&b, false, false)
	out := b.String()
	if strings.Contains(out, "/Users/example") {
		t.Fatalf("absolute home path printed:\n%s", out)
	}
	if !strings.Contains(out, "~/.downshift/") || !strings.Contains(out, "…") {
		t.Fatalf("want a home-relative path, cut in the middle:\n%s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if n := utf8.RuneCountInString(line); n > 60 {
			t.Errorf("line is %d columns, terminal is 60: %q", n, line)
		}
	}
}

func TestDoctorText_NoWidthMeansDefault(t *testing.T) {
	long := strings.Repeat("a", 200)
	rep := &doctorReport{}
	rep.add(doctorCheck{ID: "catalog", Status: doctorOK, Detail: long})
	var b bytes.Buffer
	rep.writeText(&b, false, false)
	for _, line := range strings.Split(b.String(), "\n") {
		if n := utf8.RuneCountInString(line); n > defaultTextWidth {
			t.Fatalf("line is %d columns, default is %d: %q", n, defaultTextWidth, line)
		}
	}
}

func TestMiddleTruncate(t *testing.T) {
	if got := middleTruncate("short", 10); got != "short" {
		t.Errorf("short text changed: %q", got)
	}
	got := middleTruncate("0123456789abcdefghij", 9)
	if got != "0123…ghij" || utf8.RuneCountInString(got) != 9 {
		t.Errorf("middleTruncate = %q", got)
	}
}

func TestDoctorQuiet_OnlyFailuresAndWarnings(t *testing.T) {
	rep := &doctorReport{}
	rep.add(doctorCheck{ID: "version", Status: doctorOK, Detail: "dev"})
	rep.add(doctorCheck{ID: "catalog", Status: doctorOK, Detail: "embedded"})
	rep.add(doctorCheck{ID: "prices", Status: doctorWarn, Detail: "old", Fix: "upgrade"})
	var b bytes.Buffer
	rep.writeText(&b, false, true)
	out := b.String()
	if strings.Contains(out, "version") || strings.Contains(out, "catalog") {
		t.Errorf("quiet prints ok checks:\n%s", out)
	}
	if !strings.Contains(out, "prices") || !strings.Contains(out, "fix: upgrade") || !strings.Contains(out, "no failures, 1 warnings") {
		t.Errorf("quiet lost the warning:\n%s", out)
	}

	b.Reset()
	rep.writeText(&b, false, false)
	if !strings.Contains(b.String(), "version") {
		t.Errorf("without --quiet every check prints:\n%s", b.String())
	}
}

func TestDoctorQuiet_AllPassedCount(t *testing.T) {
	rep := &doctorReport{}
	for _, id := range []string{"version", "catalog", "prices"} {
		rep.add(doctorCheck{ID: id, Status: doctorOK, Detail: "ok"})
	}
	var b bytes.Buffer
	rep.writeText(&b, false, true)
	out := b.String()
	if !strings.Contains(out, "all 3 checks passed") || strings.Contains(out, "✓") {
		t.Errorf("quiet with nothing to report:\n%s", out)
	}
}

// JSON keeps the absolute paths and carries no text-only field.
func TestDoctorJSON_UnchangedByTextOptions(t *testing.T) {
	f := newDoctorFixture(t)
	rep := buildDoctorReport(f.env)
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), f.home) {
		t.Errorf("json lost the absolute path %s", f.home)
	}
	if strings.Contains(string(b), "\"home\"") || strings.Contains(string(b), "\"width\"") || strings.Contains(string(b), "\"quiet\"") {
		t.Errorf("text-only field leaked into json: %s", b)
	}
}

// A long path gives up its middle first; the words around it stay whole.
func TestFitWords_PathBeforeProse(t *testing.T) {
	const s = "install -m 0755 /Users/example/build/output/downshift-binary-with-a-long-name ~/go/bin/downshift"
	got := fitWords(s, 70)
	if utf8.RuneCountInString(got) > 70 {
		t.Fatalf("%d runes, want at most 70: %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasPrefix(got, "install -m 0755 ") || !strings.HasSuffix(got, " ~/go/bin/downshift") {
		t.Errorf("prose or the short path was cut: %q", got)
	}
}
