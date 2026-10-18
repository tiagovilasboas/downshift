// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package scoreindex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

// Fixture shapes were re-checked against the live responses on 2026-10-13:
// Epoch ECI CSV columns and the OpenRouter benchmarks.artificial_analysis
// object. Haiku 5.5 is absent from Epoch ECI and present in OpenRouter.
const epochCSV = `Model,Display name,eci,eci_ci_low,eci_ci_high,date,Organization,Country (of organization),Model accessibility,Accessibility group,model_versions
Claude Opus 5.5,Claude Opus 5.5,167.33,164.06,171.68,2026-09-22,Anthropic,United States of America,API access,Closed weights,
GPT-6 Astra,GPT-6 Astra,166.45,163.34,171.46,2026-09-03,OpenAI,United States of America,API access,Closed weights,
Claude Sonnet 5.5,Claude Sonnet 5.5,165.03,161.68,169.06,2026-09-28,Anthropic,United States of America,API access,Closed weights,
Claude Fable 5.1,Claude Fable 5.1,164.7,161.63,168.81,2026-09-01,Anthropic,United States of America,API access,Closed weights,
Gemini 3.8 Flash,Gemini 3.8 Flash,156.71,154.41,160.26,2026-09-02,Google DeepMind,United States of America,API access,Closed weights,
GPT-6 Luna,GPT-6 Luna,156.28,153.67,158.75,2026-09-22,OpenAI,United States of America,API access,Closed weights,
Grok 4.5,Grok 4.5,153.92,151,156,2026-06-01,xAI,United States of America,API access,Closed weights,
`

const openRouterJSON = `{"data":[
 {"id":"anthropic/claude-opus-5.5","benchmarks":{"design_arena":[],"artificial_analysis":{"intelligence_index":57.6,"coding_index":null,"agentic_index":null}}},
 {"id":"anthropic/claude-opus-5.5:batch","benchmarks":{"artificial_analysis":{"intelligence_index":1.0}}},
 {"id":"anthropic/claude-sonnet-5.5","benchmarks":{"artificial_analysis":{"intelligence_index":56.0,"coding_index":null,"agentic_index":null}}},
 {"id":"anthropic/claude-fable-5.1","benchmarks":{"artificial_analysis":{"intelligence_index":53.4,"coding_index":81.6,"agentic_index":57.9}}},
 {"id":"anthropic/claude-haiku-5.5","benchmarks":{"artificial_analysis":{"intelligence_index":43.4,"coding_index":null,"agentic_index":null}}},
 {"id":"openai/gpt-6-luna","benchmarks":{"artificial_analysis":{"intelligence_index":38.1}}},
 {"id":"google/gemini-3.8-flash","benchmarks":{"artificial_analysis":{"intelligence_index":null}}},
 {"id":"meta/muse-wonder","benchmarks":{"design_arena":[]}},
 {"id":"x-ai/unmapped-model","benchmarks":{"artificial_analysis":{"intelligence_index":20}}}
]}`

func stateDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOWNSHIFT_STATE_DIR", dir)
	t.Setenv("DOWNSHIFT_SCORES", "")
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(dir, "none.json"))
	return dir
}

// fakeUpstream serves the Epoch CSV and the OpenRouter JSON.
type fakeUpstream struct {
	mu       sync.Mutex
	srv      *httptest.Server
	epochCSV string
	orJSON   string
	etag     string
	epochErr int // when nonzero, Epoch answers this status
	orErr    int
	gotINM   []string
	gotUA    []string
	hits     int
}

func newUpstream(t *testing.T) *fakeUpstream {
	t.Helper()
	u := &fakeUpstream{epochCSV: epochCSV, orJSON: openRouterJSON, etag: `"abc123"`}
	mux := http.NewServeMux()
	mux.HandleFunc("/eci_scores.csv", func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.hits++
		u.gotINM = append(u.gotINM, r.Header.Get("If-None-Match"))
		u.gotUA = append(u.gotUA, r.UserAgent())
		if u.epochErr != 0 {
			http.Error(w, "boom", u.epochErr)
			return
		}
		if inm := r.Header.Get("If-None-Match"); inm != "" && inm == u.etag {
			w.Header().Set("ETag", u.etag)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", u.etag)
		w.Header().Set("Content-Type", "text/csv")
		_, _ = w.Write([]byte(u.epochCSV))
	})
	mux.HandleFunc("/api/v1/models", func(w http.ResponseWriter, r *http.Request) {
		u.mu.Lock()
		defer u.mu.Unlock()
		u.hits++
		u.gotUA = append(u.gotUA, r.UserAgent())
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "no auth expected", http.StatusBadRequest)
			return
		}
		if u.orErr != 0 {
			http.Error(w, "boom", u.orErr)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(u.orJSON))
	})
	u.srv = httptest.NewServer(mux)
	t.Cleanup(u.srv.Close)
	return u
}

func (u *fakeUpstream) sources(etag string) []Source {
	m := DefaultIDMap()
	return []Source{
		&Epoch{URL: u.srv.URL + "/eci_scores.csv", Map: m, UserAgent: userAgent("test"), IfNoneMatch: etag},
		&OpenRouter{URL: u.srv.URL + "/api/v1/models", Map: m, UserAgent: userAgent("test")},
	}
}

func claudeSessions(ids ...string) map[string][]string {
	return map[string][]string{"claude-code": ids}
}

func mustRefresh(t *testing.T, u *fakeUpstream, sessions map[string][]string) *Report {
	t.Helper()
	etag := ""
	if prev, err := Load(mustPath(t)); err == nil {
		if si, ok := prev.source(SourceEpoch); ok && si.Rows > 0 {
			etag = si.ETag
		}
	}
	rep, err := Refresh(context.Background(), Options{Sources: u.sources(etag), Sessions: sessions})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return rep
}

func mustPath(t *testing.T) string {
	t.Helper()
	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func scoreOf(f *File, source, harness, id string) (float64, bool) {
	for _, r := range f.Rows {
		if r.Source == source && r.Harness == harness && r.ID == id {
			return r.Score, true
		}
	}
	return 0, false
}

// ---- id map -----------------------------------------------------------

func TestIDMapEmbeddedIsValidAndCatalogBacked(t *testing.T) {
	stateDir(t) // no user catalog override
	m, err := ParseIDMap(embeddedIDMap)
	if err != nil {
		t.Fatal(err)
	}
	cat := catalog.Load()
	for _, e := range m.Models {
		for _, tg := range e.Targets {
			if _, ok := core.CapsFor(tg.Harness); !ok {
				t.Errorf("%s: unknown harness %q", e.Name, tg.Harness)
			}
			if !cat.IsExactID(tg.Harness, tg.ID) {
				t.Errorf("%s: %s/%s is not an exact id in the embedded catalog", e.Name, tg.Harness, tg.ID)
			}
		}
	}
}

func TestIDMapLookups(t *testing.T) {
	m := DefaultIDMap()
	has := func(ts []Target, harness, id string) bool {
		for _, t := range ts {
			if t.Harness == harness && t.ID == id {
				return true
			}
		}
		return false
	}
	epoch := []struct {
		in      string
		harness string
		id      string // "" means unmapped
	}{
		{"Claude Haiku 5.5", "claude-code", "claude-haiku-5-5"},
		{"claude haiku 5.5", "claude-code", "claude-haiku-5-5"},
		{"claude-haiku-5-5_high", "claude-code", "claude-haiku-5-5"},
		{"claude-fable-5-1_xhigh", "claude-code", "claude-fable-5-1"},
		{"Claude Sonnet 5.5", "cursor", "claude-sonnet-5-5-high"},
		{"GPT-6 Luna", "codex", "gpt-6-luna"},
		{"Gemini 3.8 Flash", "antigravity", "flash"},
		{"Claude Haiku", "", ""},         // no prefix guessing
		{"Claude Haiku 5.5 Pro", "", ""}, // no prefix guessing
		{"Grok 4.5", "", ""},
	}
	for _, c := range epoch {
		got := m.LookupEpoch(c.in)
		if c.id == "" {
			if len(got) != 0 {
				t.Errorf("epoch %q: want unmapped, got %v", c.in, got)
			}
			continue
		}
		if !has(got, c.harness, c.id) {
			t.Errorf("epoch %q: want %s/%s in %v", c.in, c.harness, c.id, got)
		}
	}
	or := []struct {
		in      string
		harness string
		id      string
	}{
		{"anthropic/claude-haiku-5.5", "claude-code", "claude-haiku-5-5"},
		{"anthropic/claude-opus-5.5", "kirocrew", "claude-opus-5-5"},
		{"anthropic/claude-haiku-5.5:batch", "", ""},
		{"claude-haiku-5.5", "", ""},
		{"anthropic/claude-haiku", "", ""},
	}
	for _, c := range or {
		got := m.LookupOpenRouter(c.in)
		if c.id == "" {
			if len(got) != 0 {
				t.Errorf("openrouter %q: want unmapped, got %v", c.in, got)
			}
			continue
		}
		if !has(got, c.harness, c.id) {
			t.Errorf("openrouter %q: want %s/%s in %v", c.in, c.harness, c.id, got)
		}
	}
}

func TestIDMapRejectsDuplicatesAndBadDocs(t *testing.T) {
	bad := map[string]string{
		"duplicate epoch name": `{"version":1,"models":[
			{"name":"a","epoch":["X"],"ids":[{"harness":"h","id":"a"}]},
			{"name":"b","epoch":["x_high"],"ids":[{"harness":"h","id":"b"}]}]}`,
		"no ids":  `{"version":1,"models":[{"name":"a","epoch":["X"],"ids":[]}]}`,
		"version": `{"version":2,"models":[]}`,
		"json":    `{`,
	}
	for name, doc := range bad {
		if _, err := ParseIDMap([]byte(doc)); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

// ---- sources ----------------------------------------------------------

func TestEpochParsesRowsAndSendsValidators(t *testing.T) {
	u := newUpstream(t)
	src := u.sources("")[0].(*Epoch)
	rows, err := src.Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]float64{}
	for _, r := range rows {
		byKey[r.Harness+"/"+r.ID] = r.Score
		if r.License != "CC-BY-4.0" || r.URL == "" {
			t.Errorf("row %+v lacks license/url", r)
		}
	}
	for k, want := range map[string]float64{
		"claude-code/claude-opus-5-5":   167.33,
		"claude-code/claude-sonnet-5-5": 165.03,
		"claude-code/claude-fable-5-1":  164.7,
		"codex/gpt-6-luna":              156.28,
		"antigravity/flash":             156.71,
		"kirocrew/claude-opus-5-5":      167.33,
	} {
		if byKey[k] != want {
			t.Errorf("%s = %v, want %v", k, byKey[k], want)
		}
	}
	if _, ok := byKey["claude-code/claude-haiku-5-5"]; ok {
		t.Error("Epoch has no Haiku 5.5 row; none must be invented")
	}
	if src.ETag() != `"abc123"` {
		t.Errorf("ETag = %q", src.ETag())
	}
	if ua := u.gotUA[0]; !strings.HasPrefix(ua, "downshift/test") {
		t.Errorf("User-Agent = %q", ua)
	}
}

func TestEpochNotModified(t *testing.T) {
	u := newUpstream(t)
	src := u.sources(`"abc123"`)[0].(*Epoch)
	_, err := src.Fetch(context.Background(), nil)
	if !errors.Is(err, ErrNotModified) {
		t.Fatalf("err = %v, want ErrNotModified", err)
	}
	if u.gotINM[0] != `"abc123"` {
		t.Errorf("If-None-Match = %q", u.gotINM[0])
	}
	if src.ETag() != `"abc123"` {
		t.Errorf("ETag after 304 = %q", src.ETag())
	}
}

func TestSourcesRejectEmptyAndBadBodies(t *testing.T) {
	m := DefaultIDMap()
	for name, body := range map[string]string{
		"empty":          "",
		"header only":    "Model,Display name,eci\n",
		"no eci column":  "Model,Display name\nClaude Opus 5.5,Claude Opus 5.5\n",
		"nothing mapped": "Model,eci\nGrok 4.5,153.9\n",
		"bad eci":        "Model,eci\nClaude Opus 5.5,n/a\n",
	} {
		if _, err := parseEpoch([]byte(body), m, time.Now()); err == nil {
			t.Errorf("epoch %s: want error", name)
		}
	}
	for name, body := range map[string]string{
		"empty":    "",
		"no data":  `{"data":[]}`,
		"not json": `<html>`,
		"no index": `{"data":[{"id":"anthropic/claude-haiku-5.5","benchmarks":{}}]}`,
	} {
		if _, err := parseOpenRouter([]byte(body), m, time.Now()); err == nil {
			t.Errorf("openrouter %s: want error", name)
		}
	}
}

func TestOpenRouterParsesAAIndexAndNoAuth(t *testing.T) {
	u := newUpstream(t)
	rows, err := u.sources("")[1].Fetch(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	byKey := map[string]float64{}
	for _, r := range rows {
		byKey[r.Harness+"/"+r.ID] = r.Score
		if !strings.Contains(r.License, "Artificial Analysis") {
			t.Errorf("license %q must name Artificial Analysis", r.License)
		}
	}
	for k, want := range map[string]float64{
		"claude-code/claude-haiku-5-5":  43.4,
		"claude-code/claude-opus-5-5":   57.6, // not the :batch row (1.0)
		"claude-code/claude-sonnet-5-5": 56.0,
		"claude-code/claude-fable-5-1":  53.4,
		"codex/gpt-6-luna":              38.1,
	} {
		if byKey[k] != want {
			t.Errorf("%s = %v, want %v", k, byKey[k], want)
		}
	}
	if _, ok := byKey["antigravity/flash"]; ok {
		t.Error("null intelligence_index must not produce a row")
	}
}

func TestRedirectToAnotherHostRefused(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(epochCSV))
	}))
	defer other.Close()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/eci_scores.csv", http.StatusFound)
	}))
	defer src.Close()
	// 127.0.0.1 with different ports are different hosts.
	e := &Epoch{URL: src.URL, Map: DefaultIDMap(), UserAgent: "t"}
	if _, err := e.Fetch(context.Background(), nil); err == nil {
		t.Fatal("redirect to another host must fail")
	}
}

func TestBodyCap(t *testing.T) {
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("a"), epochMaxBody+10))
	}))
	defer big.Close()
	e := &Epoch{URL: big.URL, Map: DefaultIDMap(), UserAgent: "t"}
	if _, err := e.Fetch(context.Background(), nil); err == nil {
		t.Fatal("oversized body must fail")
	}
}

// ---- refresh, cache, selection ---------------------------------------

func TestRefreshWritesRowsPerSourceAndSelectsByCoverage(t *testing.T) {
	stateDir(t)
	u := newUpstream(t)

	// Epoch lacks Haiku 5.5: with Haiku enabled, OpenRouter covers more.
	rep := mustRefresh(t, u, claudeSessions("claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"))
	if got := rep.File.SelectedSource; got != SourceOpenRouter {
		t.Fatalf("selected = %q, want %q", got, SourceOpenRouter)
	}
	if _, ok := scoreOf(rep.File, SourceEpoch, "claude-code", "claude-opus-5-5"); !ok {
		t.Error("rows of both sources must be cached side by side")
	}
	if rep.File.Version != 1 || rep.File.Updated.IsZero() {
		t.Errorf("header = %+v", rep.File)
	}
	// The router sees only the selected source's scale.
	if v, ok := Lookup("claude-code", "claude-opus-5-5"); !ok || v != 57.6 {
		t.Errorf("Lookup opus = %v,%v want 57.6 (OpenRouter AA), not the ECI scale", v, ok)
	}
	if v, ok := Lookup("claude-code", "claude-haiku-5-5"); !ok || v != 43.4 {
		t.Errorf("Lookup haiku = %v,%v want 43.4", v, ok)
	}
	if _, ok := Lookup("claude-code", "no-such-model"); ok {
		t.Error("unknown id must be uncovered")
	}

	// Without Haiku the sources tie on coverage: priority (Epoch) wins.
	rep = mustRefresh(t, u, claudeSessions("claude-sonnet-5-5", "claude-opus-5-5"))
	if got := rep.File.SelectedSource; got != SourceEpoch {
		t.Fatalf("tie: selected = %q, want %q", got, SourceEpoch)
	}
	if v, ok := Lookup("claude-code", "claude-opus-5-5"); !ok || v != 167.33 {
		t.Errorf("Lookup opus = %v,%v want 167.33", v, ok)
	}
	if _, ok := Lookup("claude-code", "claude-haiku-5-5"); ok {
		t.Error("an id with no row in the selected source must be uncovered, even if another source has it")
	}

	// Coverage is across harnesses: codex ids only exist in both, antigravity
	// flash only in Epoch (OpenRouter index is null).
	rep = mustRefresh(t, u, map[string][]string{
		"claude-code": {"claude-haiku-5-5"},
		"antigravity": {"flash", "pro"},
		"cursor":      {"inherit"},
	})
	// openrouter: haiku(1) ; epoch: flash(1) -> tie -> epoch
	if rep.File.SelectedSource != SourceEpoch {
		t.Fatalf("selected = %q", rep.File.SelectedSource)
	}

	// No session ids at all: highest priority source with rows.
	rep = mustRefresh(t, u, nil)
	if rep.File.SelectedSource != SourceEpoch {
		t.Fatalf("no sessions: selected = %q", rep.File.SelectedSource)
	}
}

func TestSelectSourceSkipsSourceWithoutRows(t *testing.T) {
	f := &File{
		Sources: []SourceInfo{{Name: SourceEpoch}, {Name: SourceOpenRouter}},
		Rows:    []Row{{ID: "x", Harness: "h", Score: 1, Source: SourceOpenRouter}},
	}
	if got := SelectSource(f, map[string][]string{"h": {"y"}}); got != SourceOpenRouter {
		t.Errorf("got %q", got)
	}
	if got := SelectSource(&File{}, nil); got != "" {
		t.Errorf("empty file: got %q", got)
	}
}

func TestFailedOrEmptyFetchNeverReplacesCache(t *testing.T) {
	dir := stateDir(t)
	u := newUpstream(t)
	mustRefresh(t, u, claudeSessions("claude-opus-5-5"))
	path := filepath.Join(dir, FileName)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]func(){
		"http 500": func() { u.epochErr, u.orErr = 500, 500 },
		"empty":    func() { u.epochErr, u.orErr = 0, 0; u.epochCSV, u.orJSON = "", `{"data":[]}` },
		"garbage":  func() { u.epochCSV, u.orJSON = "<html>nope</html>", "not json" },
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			u.epochCSV, u.orJSON, u.epochErr, u.orErr = epochCSV, openRouterJSON, 0, 0
			setup()
			_, err := Refresh(context.Background(), Options{Sources: u.sources(""), Sessions: claudeSessions("claude-opus-5-5")})
			if err == nil {
				t.Fatal("want error when no source returns scores")
			}
			after, rerr := os.ReadFile(path)
			if rerr != nil || !bytes.Equal(before, after) {
				t.Fatal("cache file changed after a failed refresh")
			}
		})
	}
	u.epochCSV, u.orJSON, u.epochErr, u.orErr = epochCSV, openRouterJSON, 0, 0

	// No cache and a failing fetch: still no file.
	empty := t.TempDir()
	p := filepath.Join(empty, FileName)
	u.epochErr, u.orErr = 503, 503
	if _, err := Refresh(context.Background(), Options{Path: p, Sources: u.sources("")}); err == nil {
		t.Fatal("want error")
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("failed first refresh must not create the cache, stat err = %v", err)
	}
}

func TestPartialFailureKeepsPreviousRowsOfThatSource(t *testing.T) {
	stateDir(t)
	u := newUpstream(t)
	mustRefresh(t, u, claudeSessions("claude-opus-5-5"))

	u.epochErr = 500
	u.orJSON = strings.Replace(openRouterJSON, "57.6", "58.0", 1)
	rep, err := Refresh(context.Background(), Options{Sources: u.sources(""), Sessions: claudeSessions("claude-opus-5-5")})
	if err != nil {
		t.Fatal(err)
	}
	if v, ok := scoreOf(rep.File, SourceEpoch, "claude-code", "claude-opus-5-5"); !ok || v != 167.33 {
		t.Errorf("stale Epoch row lost: %v %v", v, ok)
	}
	if v, _ := scoreOf(rep.File, SourceOpenRouter, "claude-code", "claude-opus-5-5"); v != 58.0 {
		t.Errorf("OpenRouter not updated: %v", v)
	}
	var info SourceInfo
	for _, s := range rep.File.Sources {
		if s.Name == SourceEpoch {
			info = s
		}
	}
	if !strings.HasPrefix(info.Status, "error:") || info.Rows == 0 || info.ETag != `"abc123"` {
		t.Errorf("Epoch info = %+v", info)
	}
}

func TestETag304KeepsRowsAndAdvancesFetched(t *testing.T) {
	stateDir(t)
	u := newUpstream(t)
	t0 := time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)
	clock := t0
	opts := func() Options {
		etag := ""
		if f, err := Load(mustPath(t)); err == nil {
			if si, ok := f.source(SourceEpoch); ok {
				etag = si.ETag
			}
		}
		return Options{Sources: u.sources(etag), Sessions: claudeSessions("claude-opus-5-5"), Now: func() time.Time { return clock }}
	}
	if _, err := Refresh(context.Background(), opts()); err != nil {
		t.Fatal(err)
	}
	clock = t0.Add(26 * time.Hour)
	// Epoch would now serve different bytes, but the validator matches: 304.
	u.epochCSV = "Model,eci\nClaude Opus 5.5,1.0\n"
	rep, err := Refresh(context.Background(), opts())
	if err != nil {
		t.Fatal(err)
	}
	if got := u.gotINM[len(u.gotINM)-1]; got != `"abc123"` {
		t.Fatalf("second request If-None-Match = %q", got)
	}
	var info SourceInfo
	for _, s := range rep.File.Sources {
		if s.Name == SourceEpoch {
			info = s
		}
	}
	if info.Status != "not-modified" || !info.Fetched.Equal(clock) || info.ETag != `"abc123"` {
		t.Errorf("Epoch info = %+v", info)
	}
	if v, _ := scoreOf(rep.File, SourceEpoch, "claude-code", "claude-opus-5-5"); v != 167.33 {
		t.Errorf("304 must keep the cached rows, got %v", v)
	}
	if !rep.File.Updated.Equal(clock) {
		t.Errorf("Updated = %v", rep.File.Updated)
	}
}

func TestDefaultSourcesCarryCachedETag(t *testing.T) {
	src := DefaultSources(nil, `"v1"`, "9.9")
	e, ok := src[0].(*Epoch)
	if !ok || e.IfNoneMatch != `"v1"` || e.URL != EpochURL {
		t.Fatalf("epoch source = %+v", src[0])
	}
	if src[0].Name() != SourceEpoch || src[1].Name() != SourceOpenRouter {
		t.Error("priority order must be Epoch then OpenRouter")
	}
	if !strings.Contains(e.UserAgent, "downshift/9.9") {
		t.Errorf("UA = %q", e.UserAgent)
	}
}

func TestAtomicWriteLeavesNoTempAndKeepsOldOnFailure(t *testing.T) {
	dir := stateDir(t)
	u := newUpstream(t)
	mustRefresh(t, u, claudeSessions("claude-opus-5-5"))
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") || e.Name() == lockName {
			t.Errorf("leftover %s", e.Name())
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	var f File
	if err := json.Unmarshal(data, &f); err != nil || f.SelectedSource == "" {
		t.Fatalf("cache not valid JSON: %v", err)
	}

	// The rename target is a directory: the write fails, no temp remains.
	bad := filepath.Join(t.TempDir(), FileName)
	if err := os.Mkdir(bad, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(bad, &f); err == nil {
		t.Fatal("want error")
	}
	left, _ := os.ReadDir(filepath.Dir(bad))
	for _, e := range left {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("temp file %s left behind", e.Name())
		}
	}
}

func TestConcurrentRefreshIsLocked(t *testing.T) {
	dir := stateDir(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		<-release
		_, _ = w.Write([]byte(epochCSV))
	}))
	defer slow.Close()
	src := func() []Source {
		return []Source{&Epoch{URL: slow.URL, Map: DefaultIDMap(), UserAgent: "t"}}
	}
	done := make(chan error, 1)
	go func() {
		_, err := Refresh(context.Background(), Options{Sources: src()})
		done <- err
	}()
	<-started
	_, err := Refresh(context.Background(), Options{Sources: src()})
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("second refresh err = %v, want ErrLocked", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, lockName)); !os.IsNotExist(err) {
		t.Error("lock must be released")
	}
}

func TestStaleLockIsReplaced(t *testing.T) {
	dir := stateDir(t)
	u := newUpstream(t)
	lock := filepath.Join(dir, lockName)
	if err := os.WriteFile(lock, []byte("999\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Refresh(context.Background(), Options{Sources: u.sources("")}); !errors.Is(err, ErrLocked) {
		t.Fatalf("fresh foreign lock: err = %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	if _, err := Refresh(context.Background(), Options{Sources: u.sources("")}); err != nil {
		t.Fatalf("stale lock must be replaced: %v", err)
	}
}

func TestDisabledIsNoOp(t *testing.T) {
	dir := stateDir(t)
	u := newUpstream(t)
	mustRefresh(t, u, claudeSessions("claude-opus-5-5"))
	hits := u.hits
	t.Setenv("DOWNSHIFT_SCORES", "off")
	if _, err := Refresh(context.Background(), Options{Sources: u.sources("")}); !errors.Is(err, ErrDisabled) {
		t.Fatalf("err = %v", err)
	}
	if u.hits != hits {
		t.Error("disabled refresh must not touch the network")
	}
	if _, ok := Lookup("claude-code", "claude-opus-5-5"); ok {
		t.Error("disabled Lookup must report unknown")
	}
	st, err := BuildStatus("", nil, time.Now())
	if err != nil || !st.Disabled {
		t.Fatalf("status = %+v, %v", st, err)
	}
	var b bytes.Buffer
	st.WriteText(&b)
	if !strings.Contains(b.String(), "off") {
		t.Errorf("text = %q", b.String())
	}
	_ = dir
}

// ---- hook ---------------------------------------------------------------

func TestScoreHookIsRegisteredAndFollowsTheFile(t *testing.T) {
	stateDir(t)
	if core.ScoreHook == nil {
		t.Fatal("core.ScoreHook must be registered by scoreindex")
	}
	if _, ok := core.ScoreHook("claude-code", "claude-opus-5-5"); ok {
		t.Fatal("no cache: unknown")
	}
	u := newUpstream(t)
	mustRefresh(t, u, claudeSessions("claude-opus-5-5", "claude-sonnet-5-5"))
	if v, ok := core.ScoreHook("claude-code", "claude-opus-5-5"); !ok || v != 167.33 {
		t.Errorf("hook = %v,%v", v, ok)
	}
	u.epochCSV = strings.Replace(epochCSV, "167.33", "170.5", 1)
	u.etag = `"new"`
	mustRefresh(t, u, claudeSessions("claude-opus-5-5", "claude-sonnet-5-5"))
	if v, ok := core.ScoreHook("claude-code", "claude-opus-5-5"); !ok || v != 170.5 {
		t.Errorf("hook after rewrite = %v,%v", v, ok)
	}
	if err := os.WriteFile(mustPath(t), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := core.ScoreHook("claude-code", "claude-opus-5-5"); ok {
		t.Error("an unreadable cache must fail open (unknown)")
	}
}

// ---- status -------------------------------------------------------------

func TestStatusReportsCoverageAndAttribution(t *testing.T) {
	stateDir(t)
	u := newUpstream(t)
	sessions := map[string][]string{
		"claude-code": {"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"},
		"cursor":      {"inherit", "composer-2.5", "claude-sonnet-5-5-high"},
		"codex":       {"gpt-6-luna"},
		"kirocrew":    nil,
	}
	rep := mustRefresh(t, u, sessions)
	now := rep.File.Updated.Add(3 * time.Hour)
	st, err := BuildStatus("", sessions, now)
	if err != nil {
		t.Fatal(err)
	}
	if st.SelectedSource != SourceOpenRouter || st.AgeSeconds != 3*3600 {
		t.Fatalf("status = %+v", st)
	}
	var b bytes.Buffer
	st.WriteText(&b)
	out := b.String()
	for _, want := range []string{"selected source: openrouter-aa", "43.40", "uncovered", "kirocrew: no session list", "Epoch AI", "Artificial Analysis"} {
		if !strings.Contains(out, want) {
			t.Errorf("status text lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "inherit") {
		t.Error("inherit is not a model")
	}
	line := lineWith(out, "composer-2.5")
	if !strings.Contains(line, "uncovered") {
		t.Errorf("composer line = %q", line)
	}
	// 6 distinct session ids (inherit excluded): openrouter covers 5, epoch 4.
	for _, s := range st.Sources {
		switch s.Name {
		case SourceOpenRouter:
			if s.Covered != 5 || s.Total != 6 {
				t.Errorf("openrouter coverage = %d/%d, want 5/6", s.Covered, s.Total)
			}
		case SourceEpoch:
			if s.Covered != 4 || s.Total != 6 {
				t.Errorf("epoch coverage = %d/%d, want 4/6", s.Covered, s.Total)
			}
		}
	}

	var j bytes.Buffer
	if err := st.WriteJSON(&j); err != nil {
		t.Fatal(err)
	}
	var back Status
	if err := json.Unmarshal(j.Bytes(), &back); err != nil || back.SelectedSource != SourceOpenRouter || len(back.Harnesses) != 4 {
		t.Fatalf("json status = %v %+v", err, back)
	}

	// Session lists changed since the refresh: the status says what a refresh would do.
	st2, _ := BuildStatus("", claudeSessions("claude-sonnet-5-5", "claude-opus-5-5"), now)
	if st2.Reselect != SourceEpoch {
		t.Errorf("Reselect = %q, want %q", st2.Reselect, SourceEpoch)
	}
}

func lineWith(s, sub string) string {
	for _, l := range strings.Split(s, "\n") {
		if strings.Contains(l, sub) {
			return l
		}
	}
	return ""
}

func TestStatusMissingCache(t *testing.T) {
	stateDir(t)
	st, err := BuildStatus("", claudeSessions("claude-opus-5-5"), time.Now())
	if err != nil || st.Present {
		t.Fatalf("status = %+v %v", st, err)
	}
	var b bytes.Buffer
	st.WriteText(&b)
	if !strings.Contains(b.String(), "missing") || !strings.Contains(b.String(), "--refresh") {
		t.Errorf("text = %q", b.String())
	}
}
