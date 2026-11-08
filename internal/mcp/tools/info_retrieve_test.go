// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tiagovilasboas/downshift-ai/internal/ccr"
	"github.com/tiagovilasboas/downshift-ai/internal/execwrap"
	"github.com/tiagovilasboas/downshift-ai/internal/paths"
)

func callRetrieve(t *testing.T, args string) Result {
	t.Helper()
	res, err := retrieveTool().Handler(context.Background(), json.RawMessage(args))
	if err != nil {
		t.Fatalf("a retrieve failure must be a result, got protocol error %v", err)
	}
	return res
}

func TestRetrieveToolReturnsExactLines(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	store, err := ccr.Default()
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Put([]byte("one\ntwo FAIL\nthree\nfour FAIL\nfive\n"))
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		`{"id":"` + id + `"}`:                        "one\ntwo FAIL\nthree\nfour FAIL\nfive\n",
		`{"id":"` + id + `","grep":"FAIL","head":1}`: "two FAIL\n",
		`{"id":"` + id + `","tail":2}`:               "four FAIL\nfive\n",
		`{"id":"` + id + `","lines":"2:3"}`:          "2:two FAIL\n3:three\n",
	}
	for args, want := range cases {
		if res := callRetrieve(t, args); res.IsError || res.Text != want {
			t.Errorf("%s: got %q (error=%v), want %q", args, res.Text, res.IsError, want)
		}
	}
}

func TestRetrieveToolErrorsAreResults(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	for _, args := range []string{
		``, `{}`, `{"id":"../x"}`, `{"id":"000000000000"}`, `{"id":"000000000000","extra":1}`,
		`{"id":"000000000000","lines":"x"}`, `{"id":"000000000000","head":-1}`, `[]`,
	} {
		if res := callRetrieve(t, args); !res.IsError || res.Text == "" {
			t.Errorf("%q: want an error result, got %+v", args, res)
		}
	}
}

func TestRetrieveToolIsReadOnlyAndListed(t *testing.T) {
	tl := toolByName(t, fullDeps(), "retrieve")
	if !tl.ReadOnly || !tl.Idempotent {
		t.Errorf("retrieve must be read-only and idempotent: %+v", tl)
	}
	if err := Validate([]Tool{tl}); err != nil {
		t.Error(err)
	}
}

func TestRetrieveToolRecordsBytesReturned(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	store, err := ccr.Default()
	if err != nil {
		t.Fatal(err)
	}
	id, err := store.Put([]byte("one\ntwo FAIL\nthree\n"))
	if err != nil {
		t.Fatal(err)
	}
	res := runRetrieve([]byte(`{"id":"` + id + `","grep":"FAIL"}`))
	if res.IsError || res.Text != "two FAIL\n" {
		t.Fatalf("result = %+v", res)
	}
	events, err := execwrap.EventsPath()
	if err != nil {
		t.Fatal(err)
	}
	if got := execwrap.ReadRetrievals(events)[id]; got != int64(len("two FAIL\n")) {
		t.Fatalf("recorded %d bytes for %s, want %d", got, id, len("two FAIL\n"))
	}
	// A failed read records nothing.
	if res := runRetrieve([]byte(`{"id":"000000000000"}`)); !res.IsError {
		t.Fatalf("unknown id should fail: %+v", res)
	}
	if got := len(execwrap.ReadRetrievals(events)); got != 1 {
		t.Fatalf("failed read recorded: %v", execwrap.ReadRetrievals(events))
	}
}
