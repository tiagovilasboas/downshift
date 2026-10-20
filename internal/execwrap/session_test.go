// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"path/filepath"
	"testing"
)

const hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestWithSession(t *testing.T) {
	cases := []struct {
		name, command, session, want string
	}{
		{"adds the flag", "'/x/downshift' compact -- go test ./...", hashA, "'/x/downshift' compact --session=" + hashA + " -- go test ./..."},
		{"no session leaves it", "'/x/downshift' compact -- go test ./...", "", "'/x/downshift' compact -- go test ./..."},
		{"non-hex session leaves it", "'/x/downshift' compact -- git status", "../etc", "'/x/downshift' compact -- git status"},
		{"already has one", "'/x/downshift' compact --session=" + hashA + " -- git status", hashA, "'/x/downshift' compact --session=" + hashA + " -- git status"},
		{"not wrapped", "go test ./...", hashA, "go test ./..."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WithSession(tc.command, tc.session); got != tc.want {
				t.Fatalf("WithSession = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSummarizeSession_FiltersAndKeepsOldRecordsOut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exec-events.jsonl")
	mk := func(session string, raw, emitted int64) Event {
		return Event{TS: "2026-10-20T00:00:00Z", Family: "go_test", RawBytes: raw, EmittedBytes: emitted, ExitClass: ExitClassZero, Compacted: raw > emitted, Session: session}
	}
	for _, ev := range []Event{mk(hashA, 1000, 400), mk(hashA, 200, 200), mk("b"+hashA[1:], 5000, 100), mk("", 700, 100)} {
		if err := Append(path, ev); err != nil {
			t.Fatal(err)
		}
	}
	one, err := SummarizeSession(path, hashA)
	if err != nil {
		t.Fatal(err)
	}
	if one.Commands != 2 || one.Compacted != 1 || one.SavedBytes != 600 || one.EstimatedTokensSaved != 150 {
		t.Fatalf("session summary = %+v", one)
	}
	all, _ := Summarize(path)
	if all.Commands != 4 {
		t.Fatalf("whole-log summary lost records: %+v", all)
	}
	none, _ := SummarizeSession(path, "c"+hashA[1:])
	if none.Commands != 0 {
		t.Fatalf("unknown session must total nothing: %+v", none)
	}
}
