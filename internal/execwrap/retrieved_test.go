// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/paths"
)

const (
	idA = "aaaaaaaaaaaa"
	idB = "bbbbbbbbbbbb"
	idC = "cccccccccccc"
)

// keptRun is a compacted go-test run of raw bytes cut to 1000, kept under id.
func keptRun(id string, raw int64) Event {
	return Event{
		TS: "2026-11-01T10:00:00Z", Family: FamilyGoTest, RawBytes: raw, EmittedBytes: 1000,
		ExitClass: ExitClassZero, Compacted: true, KeptID: id,
	}
}

func retr(id string, n int64) Retrieval {
	return Retrieval{TS: "2026-11-01T10:05:00Z", ID: id, Bytes: n}
}

// writeLogs writes the runs and the retrievals into dir and returns the exec
// log path.
func writeLogs(t *testing.T, dir string, evs []Event, rets []Retrieval) string {
	t.Helper()
	path := filepath.Join(dir, EventsFileName)
	for _, ev := range evs {
		if err := Append(path, ev); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range rets {
		if err := AppendRetrieval(retrievalsBeside(path), r); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestSummarize_Retrieval(t *testing.T) {
	cases := []struct {
		name          string
		evs           []Event
		rets          []Retrieval
		wantSaved     int64
		wantRetrieved int64
		wantFamily    int64 // SavedBytesByFamily[FamilyGoTest]
	}{
		{"no retrieval", []Event{keptRun(idA, 11000)}, nil, 10000, 0, 10000},
		{"partial retrieval", []Event{keptRun(idA, 11000)}, []Retrieval{retr(idA, 3000)}, 7000, 3000, 7000},
		{"retrieved in full", []Event{keptRun(idA, 11000)}, []Retrieval{retr(idA, 10000)}, 0, 10000, 0},
		{"retrieval larger than the saving", []Event{keptRun(idA, 11000)}, []Retrieval{retr(idA, 25000)}, 0, 25000, 0},
		{"unknown id changes nothing", []Event{keptRun(idA, 11000)}, []Retrieval{retr(idB, 5000)}, 10000, 0, 10000},
		{"an id retrieved twice counts twice", []Event{keptRun(idA, 11000)}, []Retrieval{retr(idA, 2000), retr(idA, 3000)}, 5000, 5000, 5000},
		{"twice over the saving floors at zero", []Event{keptRun(idA, 11000)}, []Retrieval{retr(idA, 7000), retr(idA, 7000)}, 0, 14000, 0},
		{"floor is per run: the other run keeps its saving", []Event{keptRun(idA, 11000), keptRun(idB, 6000)}, []Retrieval{retr(idA, 40000)}, 5000, 40000, 5000},
		{"runs of an id with no kept id are untouched", []Event{keptRun("", 11000)}, []Retrieval{retr(idA, 9000)}, 10000, 0, 10000},
		{"same id kept by two runs: oldest first, each up to its saving", []Event{keptRun(idA, 11000), keptRun(idA, 6000)}, []Retrieval{retr(idA, 12000)}, 3000, 12000, 3000},
		{"same id, retrieval inside the oldest run", []Event{keptRun(idA, 11000), keptRun(idA, 6000)}, []Retrieval{retr(idA, 4000)}, 11000, 4000, 11000},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := writeLogs(t, t.TempDir(), tc.evs, tc.rets)
			s, err := Summarize(path)
			if err != nil {
				t.Fatal(err)
			}
			if s.SavedBytes != tc.wantSaved || s.RetrievedBytes != tc.wantRetrieved {
				t.Fatalf("saved=%d retrieved=%d, want %d and %d", s.SavedBytes, s.RetrievedBytes, tc.wantSaved, tc.wantRetrieved)
			}
			if got := s.SavedBytesByFamily[FamilyGoTest]; got != tc.wantFamily {
				t.Errorf("saved by family = %d, want %d", got, tc.wantFamily)
			}
			if s.EstimatedTokensSaved != tc.wantSaved/BytesPerToken {
				t.Errorf("tokens = %d, want %d", s.EstimatedTokensSaved, tc.wantSaved/BytesPerToken)
			}
			if s.Commands != int64(len(tc.evs)) {
				t.Errorf("commands = %d, want %d: a retrieval is not a command", s.Commands, len(tc.evs))
			}
		})
	}
}

func TestSummarize_FullyRetrievedRunReportsZero(t *testing.T) {
	path := writeLogs(t, t.TempDir(), []Event{keptRun(idA, 50000)}, []Retrieval{retr(idA, 49000)})
	s, err := Summarize(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.SavedBytes != 0 || s.EstimatedTokensSaved != 0 || len(s.SavedBytesByFamily) != 0 {
		t.Fatalf("a fully retrieved run must save nothing: %+v", s)
	}
	if s.RetrievedBytes != 49000 || s.Compacted != 1 {
		t.Fatalf("the retrieved bytes stay visible: %+v", s)
	}
}

func TestSummarize_FoldedSavingIsNet(t *testing.T) {
	ev := keptRun(idA, 11000)
	ev.Folded = true
	path := writeLogs(t, t.TempDir(), []Event{ev}, []Retrieval{retr(idA, 4000)})
	s, _ := Summarize(path)
	if s.SavedBytes != 6000 || s.FoldedSavedBytes != 6000 || s.FoldedRuns != 1 {
		t.Fatalf("folded = %+v", s)
	}
}

// A log written before kept_id and retrievals existed sums as it always did.
func TestSummarize_LegacyLogUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), EventsFileName)
	row := `{"ts":"2026-01-01T00:00:00Z","family":"go_test","raw_bytes":4000,"emitted_bytes":40,"exit_class":"zero","compacted":true}` + "\n"
	if err := os.WriteFile(path, []byte(row), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := Summarize(path)
	if err != nil || s.SavedBytes != 3960 || s.RetrievedBytes != 0 || s.Commands != 1 {
		t.Fatalf("summary = %+v %v", s, err)
	}
	// Retrievals with nothing to match leave it alone too.
	if err := AppendRetrieval(retrievalsBeside(path), retr(idA, 100)); err != nil {
		t.Fatal(err)
	}
	if s2, _ := Summarize(path); s2.SavedBytes != 3960 || s2.RetrievedBytes != 0 {
		t.Fatalf("summary with an unmatched retrieval = %+v", s2)
	}
}

func TestSummarize_RotatedLogs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, EventsFileName)
	// Older generation: the run and one retrieval of it.
	old := writeLogs(t, dir, []Event{keptRun(idA, 11000)}, []Retrieval{retr(idA, 3000)})
	for _, p := range []string{old, retrievalsBeside(old)} {
		if err := os.Rename(p, p+rotatedSuffix); err != nil {
			t.Fatal(err)
		}
	}
	// Current generation: another run, a second retrieval of the old run, and
	// one of the new run.
	if err := Append(path, keptRun(idB, 6000)); err != nil {
		t.Fatal(err)
	}
	for _, r := range []Retrieval{retr(idA, 2000), retr(idB, 1000)} {
		if err := AppendRetrieval(retrievalsBeside(path), r); err != nil {
			t.Fatal(err)
		}
	}
	s, err := Summarize(path)
	if err != nil {
		t.Fatal(err)
	}
	// A: 10000 - (3000+2000) = 5000. B: 5000 - 1000 = 4000.
	if s.SavedBytes != 9000 || s.RetrievedBytes != 6000 || s.Commands != 2 {
		t.Fatalf("rotated summary = %+v", s)
	}
	evs, _ := ReadEvents(path)
	if len(evs) != 2 || evs[0].RetrievedBytes != 5000 || evs[1].RetrievedBytes != 1000 {
		t.Fatalf("ReadEvents = %+v", evs)
	}
}

// A retrieval whose run rotated out of the log charges nothing.
func TestSummarize_RunRotatedAwayKeepsRetrievalHarmless(t *testing.T) {
	path := writeLogs(t, t.TempDir(), []Event{keptRun(idB, 6000)}, []Retrieval{retr(idA, 9000)})
	s, _ := Summarize(path)
	if s.SavedBytes != 5000 || s.RetrievedBytes != 0 {
		t.Fatalf("summary = %+v", s)
	}
}

func TestSummarizeSession_RetrievalFollowsItsRun(t *testing.T) {
	mine, other := keptRun(idA, 11000), keptRun(idB, 11000)
	mine.Session, other.Session = "sess-mine", "sess-other"
	path := writeLogs(t, t.TempDir(), []Event{mine, other}, []Retrieval{retr(idA, 10000), retr(idB, 1000)})
	s, err := SummarizeSession(path, "sess-mine")
	if err != nil || s.Commands != 1 || s.SavedBytes != 0 || s.RetrievedBytes != 10000 {
		t.Fatalf("mine = %+v %v", s, err)
	}
	if s, _ := SummarizeSession(path, "sess-other"); s.SavedBytes != 9000 || s.RetrievedBytes != 1000 {
		t.Fatalf("other = %+v", s)
	}
}

func TestReadRetrievals_SkipsBadRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), RetrievalsFileName)
	body := strings.Join([]string{
		`{"ts":"2026-11-01T10:00:00Z","id":"aaaaaaaaaaaa","bytes":10}`,
		`not json`,
		`{"ts":"2026-11-01T10:00:00Z","id":"../../etc/pass","bytes":10}`,
		`{"ts":"2026-11-01T10:00:00Z","id":"bbbbbbbbbbbb","bytes":-5}`,
		`{"ts":"2026-11-01T10:00:00Z","id":"bbbbbbbbbbbb","bytes":0}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRetrievals(path)
	if err != nil || len(got) != 1 || got[0].ID != idA {
		t.Fatalf("rows = %+v %v", got, err)
	}
}

func TestNetSaved(t *testing.T) {
	for _, tc := range []struct {
		raw, emitted, retrieved, want int64
	}{
		{1000, 100, 0, 900}, {1000, 100, 400, 500}, {1000, 100, 900, 0}, {1000, 100, 5000, 0}, {100, 1000, 0, 0}, {100, 100, 50, 0},
	} {
		ev := Event{RawBytes: tc.raw, EmittedBytes: tc.emitted, RetrievedBytes: tc.retrieved}
		if got := ev.NetSaved(); got != tc.want {
			t.Errorf("NetSaved(%+v) = %d, want %d", tc, got, tc.want)
		}
	}
}

// What the log holds: the id, the byte count and the time, nothing else.
func TestRecordRetrieval_StoresOnlyIDBytesTime(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	now := time.Date(2026, 11, 1, 10, 5, 0, 0, time.UTC)
	RecordRetrieval(idA, 321, now)
	RecordRetrieval("not-an-id", 10, now) // refused
	RecordRetrieval(idB, 0, now)          // nothing returned: refused
	path, err := RetrievalsPath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ts":"2026-11-01T10:05:00Z","id":"aaaaaaaaaaaa","bytes":321}` + "\n"
	if string(raw) != want {
		t.Fatalf("log = %q, want %q", raw, want)
	}
}
