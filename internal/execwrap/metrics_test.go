// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMetrics_AppendSummarize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", EventsFileName)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	for _, r := range []Result{
		{Family: FamilyGoTest, RawBytes: 4000, EmittedBytes: 40, ExitClass: ExitClassZero, Compacted: true},
		{Family: FamilyGitStatus, RawBytes: 100, EmittedBytes: 100, ExitClass: ExitClassNonZero},
		{Family: FamilyNone, ExitClass: ExitClassNotFound},
	} {
		if err := Append(path, EventFrom(r, now)); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Summarize(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Commands != 3 || s.Compacted != 1 || s.Failures != 2 || s.SavedBytes != 3960 || s.EstimatedTokensSaved != 990 {
		t.Fatalf("summary = %+v", s)
	}
	if s.ByFamily[FamilyGoTest] != 1 || s.ByFamily["other"] != 1 {
		t.Fatalf("by family = %v", s.ByFamily)
	}
	if s.SavedBytesByFamily[FamilyGoTest] != 3960 {
		t.Fatalf("saved bytes by family = %v", s.SavedBytesByFamily)
	}
	if s.CompactedByFamily[FamilyGoTest] != 1 {
		t.Fatalf("compacted by family = %v", s.CompactedByFamily)
	}
	if s.FailuresByFamily[FamilyGitStatus] != 1 || s.FailuresByFamily["other"] != 1 {
		t.Fatalf("failures by family = %v", s.FailuresByFamily)
	}
	if !strings.HasPrefix(s.TokenEstimate, "estimated") {
		t.Fatalf("token figure must be labelled estimated: %q", s.TokenEstimate)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// POSIX file permissions do not apply on Windows (Go reports 0666/0444),
	// so only assert the 0600 mode on Unix-like systems.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want 0600", info.Mode().Perm())
	}
}

// The record has exactly the prompt-free fields: no command, args, path or
// output can be stored because the type has nowhere to put them.
func TestMetrics_RecordIsPromptFree(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	if err := Append(path, EventFrom(Result{Family: FamilyGoTest, ExitClass: ExitClassZero}, time.Now())); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	line := string(data)
	for _, key := range []string{`"ts"`, `"family"`, `"raw_bytes"`, `"emitted_bytes"`, `"exit_class"`, `"compacted"`} {
		if !strings.Contains(line, key) {
			t.Fatalf("missing %s in %s", key, line)
		}
	}
	if strings.Count(line, `":`) != 6 {
		t.Fatalf("unexpected extra fields: %s", line)
	}
}

func TestMetrics_MissingFileIsEmpty(t *testing.T) {
	s, err := Summarize(filepath.Join(t.TempDir(), "none.jsonl"))
	if err != nil || s.Commands != 0 {
		t.Fatalf("s=%+v err=%v", s, err)
	}
}

// The optional client field rides the event to the log when set, and stays
// absent (no "client" key) when empty, thanks to omitempty. The caller fills it
// from the harness environment; here we set it directly to check the wire form.
func TestMetrics_ClientOmitempty(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), EventsFileName)
		ev := EventFrom(Result{Family: FamilyGoTest, ExitClass: ExitClassZero}, time.Now())
		ev.Client = "claude-desktop"
		if err := Append(path, ev); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"client":"claude-desktop"`) {
			t.Fatalf("line missing client key: %s", data)
		}
		evs, err := ReadEvents(path)
		if err != nil || len(evs) != 1 || evs[0].Client != "claude-desktop" {
			t.Fatalf("read back client = %+v, err = %v", evs, err)
		}
	})

	t.Run("absent when empty", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), EventsFileName)
		if err := Append(path, EventFrom(Result{Family: FamilyGoTest, ExitClass: ExitClassZero}, time.Now())); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), `"client"`) {
			t.Fatalf("empty client must be omitted, got: %s", data)
		}
	})
}

// fixtureExecLog is a small exec log of one window: compacted and raw runs, a
// failing run of each kind, every passthrough reason and a row with no reason.
const fixtureExecLog = `{"ts":"2026-11-03T10:00:00Z","family":"go-test","raw_bytes":4000,"emitted_bytes":40,"exit_class":"zero","compacted":true}
{"ts":"2026-11-03T10:01:00Z","family":"go-test","raw_bytes":3000,"emitted_bytes":300,"exit_class":"nonzero","compacted":true}
{"ts":"2026-11-03T10:02:00Z","family":"go-test","raw_bytes":50,"emitted_bytes":50,"exit_class":"zero","compacted":false,"passthrough":"small_output"}
{"ts":"2026-11-03T10:03:00Z","family":"git-status","raw_bytes":100,"emitted_bytes":100,"exit_class":"nonzero","compacted":false,"passthrough":"nonzero_exit"}
{"ts":"2026-11-03T10:04:00Z","family":"pytest","raw_bytes":900,"emitted_bytes":900,"exit_class":"zero","compacted":false,"passthrough":"unrecognized_output"}
{"ts":"2026-11-03T10:05:00Z","family":"pytest","raw_bytes":600,"emitted_bytes":600,"exit_class":"zero","compacted":false,"passthrough":"unrecognized_output"}
{"ts":"2026-11-03T10:06:00Z","family":"other","raw_bytes":0,"emitted_bytes":0,"exit_class":"not_found","compacted":false,"passthrough":"refused"}
{"ts":"2026-11-03T10:07:00Z","family":"go-test","raw_bytes":70,"emitted_bytes":70,"exit_class":"zero","compacted":false}
not json
`

func writeFixtureExecLog(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), EventsFileName)
	if err := os.WriteFile(path, []byte(fixtureExecLog), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Summarize gives, without a manual query, what the compactor evidence needs:
// the passthrough count by reason, the raw bytes passed through by family and
// the failing runs split into compacted and passed raw.
func TestSummarize_PassthroughAndFailureSplit(t *testing.T) {
	s, err := Summarize(writeFixtureExecLog(t))
	if err != nil {
		t.Fatal(err)
	}
	if s.Commands != 8 || s.Compacted != 2 {
		t.Fatalf("commands/compacted = %d/%d, want 8/2", s.Commands, s.Compacted)
	}
	wantReasons := map[string]int64{
		PassthroughSmallOutput: 1, PassthroughNonZeroExit: 1, PassthroughUnrecognized: 2,
		PassthroughRefused: 1, PassthroughNotRecorded: 1,
	}
	var total int64
	for reason, want := range wantReasons {
		if got := s.PassthroughByReason[reason]; got != want {
			t.Errorf("passthrough %q = %d, want %d", reason, got, want)
		}
		total += s.PassthroughByReason[reason]
	}
	if total != s.Commands-s.Compacted || len(s.PassthroughByReason) != len(wantReasons) {
		t.Errorf("reasons %v must add up to the %d runs not compacted", s.PassthroughByReason, s.Commands-s.Compacted)
	}
	wantRaw := map[Family]int64{FamilyGoTest: 120, FamilyGitStatus: 100, FamilyPytest: 1500, "other": 0}
	for fam, want := range wantRaw {
		if got := s.PassthroughRawBytesByFamily[fam]; got != want {
			t.Errorf("passthrough raw bytes %s = %d, want %d", fam, got, want)
		}
	}
	if s.FailuresCompacted != 1 || s.FailuresPassedRaw != 2 || s.Failures != 3 {
		t.Errorf("failures = %d (compacted %d, raw %d), want 3 (1, 2)", s.Failures, s.FailuresCompacted, s.FailuresPassedRaw)
	}
	// The totals that already existed do not move.
	if s.RawBytes != 8720 || s.EmittedBytes != 2060 || s.SavedBytes != 6660 || s.EstimatedTokensSaved != 1665 {
		t.Errorf("existing totals changed: %+v", s)
	}
}

// The new fields are omitempty: a log with no passthrough and no failure adds
// no key to the JSON, and every field that existed keeps its name.
func TestSummarize_NewFieldsAreOmittedWhenEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	if err := Append(path, EventFrom(Result{Family: FamilyGoTest, RawBytes: 400, EmittedBytes: 40, ExitClass: ExitClassZero, Compacted: true}, time.Now())); err != nil {
		t.Fatal(err)
	}
	s, err := Summarize(path)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"passthrough_by_reason", "passthrough_raw_bytes_by_family", "failures_compacted", "failures_passed_raw"} {
		if strings.Contains(string(raw), key) {
			t.Errorf("empty %s must be omitted: %s", key, raw)
		}
	}
	for _, key := range []string{"commands", "compacted", "raw_bytes", "emitted_bytes", "saved_bytes", "estimated_tokens_saved", "token_estimate", "commands_by_family", "saved_bytes_by_family", "compacted_by_family"} {
		if !strings.Contains(string(raw), `"`+key+`"`) {
			t.Errorf("existing field %s is gone: %s", key, raw)
		}
	}
}

// A session summary splits the same way, over that session's rows only.
func TestSummarizeSession_PassthroughSplit(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	now := time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC)
	for _, row := range []struct {
		session string
		r       Result
	}{
		{"a", Result{Family: FamilyPytest, RawBytes: 900, EmittedBytes: 900, ExitClass: ExitClassNonZero, Passthrough: PassthroughNonZeroExit}},
		{"b", Result{Family: FamilyPytest, RawBytes: 500, EmittedBytes: 500, ExitClass: ExitClassZero, Passthrough: PassthroughUnrecognized}},
	} {
		ev := EventFrom(row.r, now)
		ev.Session = row.session
		if err := Append(path, ev); err != nil {
			t.Fatal(err)
		}
	}
	s, err := SummarizeSession(path, "a")
	if err != nil {
		t.Fatal(err)
	}
	if s.PassthroughByReason[PassthroughNonZeroExit] != 1 || len(s.PassthroughByReason) != 1 ||
		s.PassthroughRawBytesByFamily[FamilyPytest] != 900 || s.FailuresPassedRaw != 1 || s.FailuresCompacted != 0 {
		t.Errorf("session a = %+v", s)
	}
}
