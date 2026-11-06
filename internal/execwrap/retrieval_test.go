// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package execwrap

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/ccr"
	"github.com/tiagovilasboas/downshift/internal/paths"
)

func storeWith(t *testing.T, raw string) (ccr.Store, string) {
	t.Helper()
	t.Setenv(paths.EnvStateDir, t.TempDir())
	store, err := ccr.Default()
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Put([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return store, id
}

func loggedRetrievals(t *testing.T) []Retrieval {
	t.Helper()
	path, err := RetrievalsPath()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ReadRetrievals(path)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestRetrieve_RecordsTheBytesReturned(t *testing.T) {
	raw := "one\ntwo FAIL\nthree\nfour FAIL\nfive\n"
	cases := []struct {
		name      string
		q         ccr.Query
		max       int
		want      string
		wantTrunc bool
	}{
		{"whole output", ccr.Query{}, 0, raw, false},
		{"grep", ccr.Query{Grep: "FAIL"}, 0, "two FAIL\nfour FAIL\n", false},
		{"cut at maxBytes records the cut length", ccr.Query{}, 10, raw[:10], true},
		{"under maxBytes", ccr.Query{Tail: 1}, 100, "five\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store, id := storeWith(t, raw)
			out, trunc, err := Retrieve(store, id, tc.q, tc.max)
			if err != nil || string(out) != tc.want || trunc != tc.wantTrunc {
				t.Fatalf("out=%q trunc=%v err=%v", out, trunc, err)
			}
			rows := loggedRetrievals(t)
			if len(rows) != 1 || rows[0].ID != id || rows[0].Bytes != int64(len(tc.want)) || rows[0].TS == "" {
				t.Fatalf("log = %+v, want one row for %s with %d bytes", rows, id, len(tc.want))
			}
		})
	}
}

func TestRetrieve_FailuresAndEmptyResultsRecordNothing(t *testing.T) {
	store, id := storeWith(t, "alpha\nbeta\n")
	cases := []struct {
		name string
		id   string
		q    ccr.Query
		want func(error) bool
	}{
		{"unknown id", "0123456789ab", ccr.Query{}, func(e error) bool { return errors.Is(e, ccr.ErrNotFound) }},
		{"invalid id", "../x", ccr.Query{}, func(e error) bool { return errors.Is(e, ccr.ErrInvalidID) }},
		{"bad grep", id, ccr.Query{Grep: "("}, func(e error) bool {
			var qe *QueryError
			return errors.As(e, &qe)
		}},
		{"nothing matched", id, ccr.Query{Grep: "zzz"}, func(e error) bool { return e == nil }},
	}
	for _, tc := range cases {
		out, _, err := Retrieve(store, tc.id, tc.q, 0)
		if !tc.want(err) {
			t.Errorf("%s: err = %v", tc.name, err)
		}
		if err != nil && out != nil {
			t.Errorf("%s: output with an error: %q", tc.name, out)
		}
	}
	if rows := loggedRetrievals(t); len(rows) != 0 {
		t.Fatalf("nothing reached the model, yet the log holds %+v", rows)
	}
}

// The log never holds the query, the output or a path.
func TestRetrieve_LogHoldsNoQueryNoOutput(t *testing.T) {
	store, id := storeWith(t, "secret-looking line /home/user/project/main.go\nother\n")
	if _, _, err := Retrieve(store, id, ccr.Query{Grep: "main.go"}, 0); err != nil {
		t.Fatal(err)
	}
	path, _ := RetrievalsPath()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"main.go", "secret", "/home", "grep"} {
		if strings.Contains(string(raw), leak) {
			t.Errorf("log %q contains %q", raw, leak)
		}
	}
}

// The recorded id is what a compacted run's kept_id carries, so a retrieval
// finds its run.
func TestRetrieve_MatchesTheRunThatKeptTheID(t *testing.T) {
	store, id := storeWith(t, strings.Repeat("a line of output\n", 600))
	eventsPath, err := EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	ev := keptRun(id, 10200) // 10200 - 1000 emitted = 9200 saved
	if err := Append(eventsPath, ev); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Retrieve(store, id, ccr.Query{Head: 100}, 0); err != nil { // 1700 bytes
		t.Fatal(err)
	}
	s, err := Summarize(eventsPath)
	if err != nil || s.RetrievedBytes != 1700 || s.SavedBytes != 9200-1700 {
		t.Fatalf("summary = %+v %v", s, err)
	}
	if _, _, err := Retrieve(store, id, ccr.Query{}, 0); err != nil { // the whole 10200
		t.Fatal(err)
	}
	if s, _ := Summarize(eventsPath); s.SavedBytes != 0 || s.RetrievedBytes != 1700+10200 {
		t.Fatalf("after fetching it all, summary = %+v", s)
	}
}
