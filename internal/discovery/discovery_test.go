// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
)

// fakeCat resolves exact ids and family prefixes like the real catalog.
type fakeCat struct{ entries []catalog.Entry }

func (f fakeCat) EntryFor(h, id string) (catalog.Entry, bool) {
	for _, e := range f.entries {
		if e.Harness == h && (e.ID == id || contains(e.Aliases, id)) {
			return e, true
		}
	}
	for _, e := range f.entries {
		if e.Harness == h && e.Family != "" && strings.HasPrefix(strings.ToLower(id), strings.ToLower(e.Family)) {
			return e, true
		}
	}
	return catalog.Entry{}, false
}

func (f fakeCat) IsExactID(h, id string) bool {
	for _, e := range f.entries {
		if e.Harness == h && (e.ID == id || contains(e.Aliases, id)) {
			return true
		}
	}
	return false
}

func contains(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

var claudeCat = fakeCat{entries: []catalog.Entry{
	{ID: "claude-haiku-5-5", Family: "claude-haiku", Harness: "claude-code", Tier: "small", OutputCostM: 0.5},
	{ID: "claude-sonnet-5-5", Family: "claude-sonnet", Harness: "claude-code", Tier: "mid", OutputCostM: 10},
	{ID: "claude-opus-5-5", Family: "claude-opus", Harness: "claude-code", Tier: "frontier", OutputCostM: 20},
}}

func TestOrderPicksNewestPerFamilyAndRanksByTier(t *testing.T) {
	d := func(y int) time.Time { return time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC) }
	r := Order("claude-code", []Model{
		{ID: "claude-opus-5-5", CreatedAt: d(2026)},
		{ID: "claude-haiku-4-5", CreatedAt: d(2025)},
		{ID: "claude-haiku-5-5", CreatedAt: d(2026)},
		{ID: "claude-sonnet-5-5", CreatedAt: d(2026)},
		{ID: "claude-mystery-1"},
	}, claudeCat)
	if want := []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"}; !reflect.DeepEqual(r.IDs, want) {
		t.Fatalf("ordered = %v, want %v", r.IDs, want)
	}
	if !reflect.DeepEqual(r.Superseded, []string{"claude-haiku-4-5"}) {
		t.Errorf("superseded = %v", r.Superseded)
	}
	if !reflect.DeepEqual(r.Unranked, []string{"claude-mystery-1"}) {
		t.Errorf("unranked = %v", r.Unranked)
	}
}

func TestOrderUnseenNewVersionInheritsFamilyTier(t *testing.T) {
	r := Order("claude-code", []Model{{ID: "claude-haiku-6-0"}, {ID: "claude-opus-5-5"}}, claudeCat)
	if want := []string{"claude-haiku-6-0", "claude-opus-5-5"}; !reflect.DeepEqual(r.IDs, want) {
		t.Fatalf("ordered = %v, want %v", r.IDs, want)
	}
	if len(r.FamilyOnly) != 1 || r.FamilyOnly[0].ID != "claude-haiku-6-0" || r.FamilyOnly[0].CatalogID != "claude-haiku-5-5" {
		t.Errorf("family_only = %+v", r.FamilyOnly)
	}
}

func TestOrderFallsBackToNumbersWithoutCreatedAt(t *testing.T) {
	r := Order("claude-code", []Model{{ID: "claude-sonnet-4-5"}, {ID: "claude-sonnet-4-10"}, {ID: "claude-sonnet-4-6"}}, claudeCat)
	if !reflect.DeepEqual(r.IDs, []string{"claude-sonnet-4-10"}) {
		t.Fatalf("ordered = %v, 4-10 must beat 4-6", r.IDs)
	}
}

func TestOrderNeverRanksAnIdTheCatalogDoesNotKnow(t *testing.T) {
	r := Order("codex", []Model{{ID: "gpt-9"}}, claudeCat)
	if len(r.IDs) != 0 || !reflect.DeepEqual(r.Unranked, []string{"gpt-9"}) {
		t.Fatalf("got %+v", r)
	}
}

const codexFixture = `{"models":[
 {"slug":"gpt-6-sol","display_name":"GPT-6-Sol","visibility":"list"},
 {"slug":"gpt-reserve","visibility":"hide"},
 {"slug":"gpt-6-luna","visibility":"list"}]}`

func TestParseCodexCacheKeepsOnlyListed(t *testing.T) {
	ms, err := parseCodexCache([]byte(codexFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 2 || ms[0].ID != "gpt-6-sol" || ms[1].ID != "gpt-6-luna" {
		t.Fatalf("got %+v", ms)
	}
	if _, err := parseCodexCache([]byte(`{"models":[]}`)); err == nil {
		t.Error("empty list must be an error, not 'no models'")
	}
}

func TestCodexCacheReadsFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "models_cache.json")
	if err := os.WriteFile(p, []byte(codexFixture), 0o600); err != nil {
		t.Fatal(err)
	}
	ms, err := CodexCache{Path: p}.Discover(context.Background())
	if err != nil || len(ms) != 2 {
		t.Fatalf("got %v, %v", ms, err)
	}
	if _, err := (CodexCache{Path: p + ".missing"}).Discover(context.Background()); err == nil {
		t.Error("missing file must be an error")
	}
}

func TestParseKiroListSkipsAuto(t *testing.T) {
	ms, err := parseKiroList([]byte(`{"models":[{"model_id":"auto","model_name":"auto"},{"model_id":"claude-haiku-4.5","model_name":"claude-haiku-4.5"}],"default_model":"auto"}`))
	if err != nil || len(ms) != 1 || ms[0].ID != "claude-haiku-4.5" {
		t.Fatalf("got %+v, %v", ms, err)
	}
}

func TestParseGrokListReadsOnlyTheModelBlock(t *testing.T) {
	out := "You are not authenticated.\n\nDefault model: grok-4.6\n\nAvailable models:\n  * grok-4.6 (default)\n  - grok-4.5\n"
	ms, err := parseGrokList([]byte(out))
	if err != nil || len(ms) != 2 || ms[0].ID != "grok-4.6" || ms[1].ID != "grok-4.5" {
		t.Fatalf("got %+v, %v", ms, err)
	}
	if _, err := parseGrokList([]byte("Default model: x\n")); err == nil {
		t.Error("no block must be an error")
	}
}

func TestAnthropicAPIListsWithCreatedAt(t *testing.T) {
	var gotKey, gotVer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey, gotVer = r.Header.Get("x-api-key"), r.Header.Get("anthropic-version")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-haiku-5-5","display_name":"Claude Haiku 5.5","created_at":"2026-09-01T00:00:00Z"}]}`))
	}))
	defer srv.Close()
	ms, err := AnthropicAPI{URL: srv.URL, Key: "k-test", Client: srv.Client()}.Discover(context.Background())
	if err != nil || len(ms) != 1 || ms[0].CreatedAt.Year() != 2026 {
		t.Fatalf("got %+v, %v", ms, err)
	}
	if gotKey != "k-test" || gotVer == "" {
		t.Errorf("auth headers: key=%q version=%q", gotKey, gotVer)
	}
}

func TestAnthropicAPIWithoutKeyIsSkippedNotEmpty(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	if _, err := (AnthropicAPI{}).Discover(context.Background()); err == nil {
		t.Fatal("no key must be an error so the previous cache entry is kept")
	}
}

func TestStoreMergeKeepsOtherHarnessesAndIsPrivate(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "discovered.json")
	if err := Merge(p, map[string]Entry{"codex": {Source: "a", Ranked: Ranked{IDs: []string{"x"}}}}); err != nil {
		t.Fatal(err)
	}
	if err := Merge(p, map[string]Entry{"grok": {Source: "b", Ranked: Ranked{IDs: []string{"y"}}}}); err != nil {
		t.Fatal(err)
	}
	f := Read(p)
	if f.Harnesses["codex"].IDs[0] != "x" || f.Harnesses["grok"].IDs[0] != "y" {
		t.Fatalf("merge lost an entry: %+v", f)
	}
	st, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", st.Mode().Perm())
	}
}

func TestReadInvalidIsEmpty(t *testing.T) {
	p := filepath.Join(t.TempDir(), "d.json")
	_ = os.WriteFile(p, []byte("{not json"), 0o600)
	if f := Read(p); len(f.Harnesses) != 0 {
		t.Fatalf("got %+v", f)
	}
}
