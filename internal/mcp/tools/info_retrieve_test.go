// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/ccr"
	"github.com/tiagovilasboas/downshift/internal/execwrap"
	"github.com/tiagovilasboas/downshift/internal/paths"
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

// The MCP tool goes through the same execwrap.Retrieve as the CLI: it records
// the bytes returned, the cut length when the result was cut, and nothing on a
// failure.
func TestRetrieveToolRecordsBytesReturned(t *testing.T) {
	t.Setenv(paths.EnvStateDir, t.TempDir())
	store, err := ccr.Default()
	if err != nil {
		t.Fatal(err)
	}
	big := strings.Repeat("0123456789abcdef\n", maxResultBytes/16+10)
	id, err := store.Put([]byte(big))
	if err != nil {
		t.Fatal(err)
	}
	if res := callRetrieve(t, `{"id":"`+id+`","head":2}`); res.IsError || res.Text != "0123456789abcdef\n0123456789abcdef\n" {
		t.Fatalf("head: %+v", res)
	}
	if res := callRetrieve(t, `{"id":"`+id+`"}`); res.IsError || !strings.Contains(res.Text, "cut at") {
		t.Fatalf("whole: error=%v, want the cut note", res.IsError)
	}
	callRetrieve(t, `{"id":"`+id+`","grep":"("}`)
	callRetrieve(t, `{"id":"000000000000"}`)
	path, err := execwrap.RetrievalsPath()
	if err != nil {
		t.Fatal(err)
	}
	rows, err := execwrap.ReadRetrievals(path)
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows = %+v %v", rows, err)
	}
	if rows[0].ID != id || rows[0].Bytes != 34 || rows[1].Bytes != int64(maxResultBytes) {
		t.Fatalf("rows = %+v, want 34 bytes then the cut length %d", rows, maxResultBytes)
	}
}
