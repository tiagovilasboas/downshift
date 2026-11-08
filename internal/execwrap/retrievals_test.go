// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

const (
	idA = "aaaaaaaaaaaa"
	idB = "bbbbbbbbbbbb"
)

func keptRun(id string, raw, emitted int64) Event {
	return Event{Family: "go-test", RawBytes: raw, EmittedBytes: emitted, Compacted: true, Kept: id}
}

func TestApplyRetrievals(t *testing.T) {
	tests := []struct {
		name      string
		evs       []Event
		byID      map[string]int64
		wantNet   []int64
		wantBack  []int64
		wantTotal int64 // Summary.SavedBytes
	}{
		{"nothing read back", []Event{keptRun(idA, 1000, 100)}, nil, []int64{900}, []int64{0}, 900},
		{"partial read back", []Event{keptRun(idA, 1000, 100)}, map[string]int64{idA: 300}, []int64{600}, []int64{300}, 600},
		{"read back larger than the saving", []Event{keptRun(idA, 1000, 100)}, map[string]int64{idA: 5000}, []int64{0}, []int64{5000}, 0},
		{"exactly the saving", []Event{keptRun(idA, 1000, 100)}, map[string]int64{idA: 900}, []int64{0}, []int64{900}, 0},
		{"unknown id changes nothing", []Event{keptRun(idA, 1000, 100)}, map[string]int64{idB: 400}, []int64{900}, []int64{0}, 900},
		{"run without a kept id", []Event{{RawBytes: 1000, EmittedBytes: 100, Compacted: true}}, map[string]int64{idA: 400}, []int64{900}, []int64{0}, 900},
		{"same output twice, one read", []Event{keptRun(idA, 1000, 100), keptRun(idA, 1000, 100)}, map[string]int64{idA: 1200},
			[]int64{0, 600}, []int64{900, 300}, 600},
		{"two ids stay apart", []Event{keptRun(idA, 1000, 100), keptRun(idB, 2000, 200)}, map[string]int64{idA: 100, idB: 1800},
			[]int64{800, 0}, []int64{100, 1800}, 800},
		{"run that saved nothing", []Event{keptRun(idA, 100, 100)}, map[string]int64{idA: 50}, []int64{0}, []int64{50}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			evs := ApplyRetrievals(append([]Event(nil), tc.evs...), tc.byID)
			for i, ev := range evs {
				if got := ev.NetSavedBytes(); got != tc.wantNet[i] {
					t.Errorf("run %d net saved = %d, want %d", i, got, tc.wantNet[i])
				}
				if ev.RetrievedBytes != tc.wantBack[i] {
					t.Errorf("run %d retrieved = %d, want %d", i, ev.RetrievedBytes, tc.wantBack[i])
				}
			}
			s := SummarizeEvents(evs)
			if s.SavedBytes != tc.wantTotal {
				t.Errorf("summary saved = %d, want %d", s.SavedBytes, tc.wantTotal)
			}
			var back int64
			for _, w := range tc.wantBack {
				back += w
			}
			if s.RetrievedBytes != back {
				t.Errorf("summary retrieved = %d, want %d", s.RetrievedBytes, back)
			}
		})
	}
}

// setState points the state directory at a temp dir and returns the paths of
// the exec log and the retrievals log in it.
func setState(t *testing.T) (events string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv(paths.EnvStateDir, dir)
	p, err := EventsPath()
	if err != nil {
		t.Skipf("state dir unavailable: %v", err)
	}
	if filepath.Dir(p) != dir {
		t.Skipf("state dir env not honoured (%s)", p)
	}
	return p
}

func TestRecordRetrievalRoundTrip(t *testing.T) {
	events := setState(t)
	if err := Append(events, keptRun(idA, 1000, 100)); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	RecordRetrieval(idA, 200, now)
	RecordRetrieval(idA, 150, now) // read twice: both count
	RecordRetrieval("not-an-id", 999, now)
	RecordRetrieval(idB, 0, now)

	if got := ReadRetrievals(events); len(got) != 1 || got[idA] != 350 {
		t.Fatalf("ReadRetrievals = %v, want {%s: 350}", got, idA)
	}
	s, err := Summarize(events)
	if err != nil {
		t.Fatal(err)
	}
	if s.SavedBytes != 550 || s.RetrievedBytes != 350 {
		t.Fatalf("summary saved=%d retrieved=%d, want 550 and 350", s.SavedBytes, s.RetrievedBytes)
	}
	evs, err := ReadEvents(events)
	if err != nil || len(evs) != 1 || evs[0].RetrievedBytes != 350 {
		t.Fatalf("ReadEvents = %+v err=%v", evs, err)
	}
}

func TestRetrievalsReadFromRotatedGeneration(t *testing.T) {
	events := setState(t)
	if err := Append(events, keptRun(idA, 1000, 100)); err != nil {
		t.Fatal(err)
	}
	RecordRetrieval(idA, 400, time.Now())
	rp := retrievalsPath(events)
	if err := os.Rename(rp, rp+rotatedSuffix); err != nil {
		t.Fatal(err)
	}
	RecordRetrieval(idA, 100, time.Now()) // lands in a fresh current file
	s, err := Summarize(events)
	if err != nil {
		t.Fatal(err)
	}
	if s.RetrievedBytes != 500 || s.SavedBytes != 400 {
		t.Fatalf("saved=%d retrieved=%d, want 400 and 500", s.SavedBytes, s.RetrievedBytes)
	}
}

// A session total counts only what was read back from its own runs.
func TestSummarizeSessionCountsOnlyItsOwnRetrievals(t *testing.T) {
	events := setState(t)
	mine, other := keptRun(idA, 1000, 100), keptRun(idB, 1000, 100)
	mine.Session, other.Session = "s1", "s2"
	for _, ev := range []Event{mine, other} {
		if err := Append(events, ev); err != nil {
			t.Fatal(err)
		}
	}
	RecordRetrieval(idA, 300, time.Now())
	RecordRetrieval(idB, 900, time.Now())
	s, err := SummarizeSession(events, "s1")
	if err != nil {
		t.Fatal(err)
	}
	if s.SavedBytes != 600 || s.RetrievedBytes != 300 {
		t.Fatalf("session s1 saved=%d retrieved=%d, want 600 and 300", s.SavedBytes, s.RetrievedBytes)
	}
}

func TestOldLogsWithoutRetrievalsAreUnchanged(t *testing.T) {
	events := setState(t)
	if err := Append(events, Event{Family: "go-test", RawBytes: 500, EmittedBytes: 50, Compacted: true}); err != nil {
		t.Fatal(err)
	}
	s, err := Summarize(events)
	if err != nil || s.SavedBytes != 450 || s.RetrievedBytes != 0 {
		t.Fatalf("summary = %+v err=%v", s, err)
	}
}

func TestRunReportsKeptID(t *testing.T) {
	// A kept id reaches the Result and the Event only when the run was compacted.
	r := Result{Family: "go-test", RawBytes: 1000, EmittedBytes: 100, Compacted: true, KeptID: idA}
	if ev := EventFrom(r, time.Now()); ev.Kept != idA {
		t.Fatalf("EventFrom kept = %q, want %q", ev.Kept, idA)
	}
}
