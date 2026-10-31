// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
)

// fixture is a hand-written, trimmed response (testdata/openrouter_models.json),
// not a captured live response. Tests never reach the real network.
var fixture = mustReadFixture()

func mustReadFixture() []byte {
	data, err := os.ReadFile("testdata/openrouter_models.json")
	if err != nil {
		panic(err)
	}
	return data
}

func tlsServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func findModel(t *testing.T, ms []Model, id string) Model {
	t.Helper()
	for _, m := range ms {
		if m.ID == id {
			return m
		}
	}
	t.Fatalf("model %q not found in %+v", id, ms)
	return Model{}
}

func TestOpenRouterParsesFixture(t *testing.T) {
	var gotUA string
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		_, _ = w.Write(fixture)
	})
	src := OpenRouterAPI{URL: srv.URL, Client: srv.Client()}
	ms, err := src.Discover(context.Background())
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(ms) != 6 {
		t.Fatalf("got %d models, want 6", len(ms))
	}
	haiku := findModel(t, ms, "anthropic/claude-haiku-5.5")
	if haiku.Display != "Anthropic: Claude Haiku 5.5" {
		t.Errorf("display = %q", haiku.Display)
	}
	if haiku.CreatedAt.Unix() != 1790000000 || !haiku.CreatedAt.Equal(haiku.CreatedAt.UTC()) {
		t.Errorf("created = %v", haiku.CreatedAt)
	}
	if haiku.Tier != "" {
		t.Errorf("tier must never be inferred from the listing, got %q", haiku.Tier)
	}
	if !strings.HasPrefix(gotUA, "downshift-discovery") {
		t.Errorf("user-agent = %q", gotUA)
	}
	if src.Harness() != "opencode" || src.Name() == "" {
		t.Errorf("harness = %q", src.Harness())
	}
}

func TestOpenRouterRefusesRedirectToAnotherHost(t *testing.T) {
	var hit atomic.Bool
	other := tlsServer(t, func(w http.ResponseWriter, _ *http.Request) {
		hit.Store(true)
		_, _ = w.Write(fixture)
	})
	srv := tlsServer(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusFound)
	})
	// srv.Client() trusts both test certificates; the redirect must still be refused.
	if _, err := (OpenRouterAPI{URL: srv.URL, Client: srv.Client()}).Discover(context.Background()); err == nil {
		t.Fatal("a redirect must be an error")
	}
	if hit.Load() {
		t.Fatal("the redirect target was requested")
	}
}

func TestOpenRouterRefusesPlainHTTP(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit.Store(true)
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()
	if _, err := (OpenRouterAPI{URL: srv.URL}).Discover(context.Background()); err == nil {
		t.Fatal("a plain http URL must be refused")
	}
	if hit.Load() {
		t.Fatal("plain http was requested")
	}
}

func TestOpenRouterNonJSONIsUnknown(t *testing.T) {
	srv := tlsServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>maintenance</html>")) })
	if _, err := (OpenRouterAPI{URL: srv.URL, Client: srv.Client()}).Discover(context.Background()); err == nil {
		t.Fatal("non-JSON must be an error")
	}
}

func TestOpenRouterEmptyListIsUnknown(t *testing.T) {
	srv := tlsServer(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"data":[]}`)) })
	if _, err := (OpenRouterAPI{URL: srv.URL, Client: srv.Client()}).Discover(context.Background()); err == nil {
		t.Fatal("an empty list must be an error so the previous cache entry is kept")
	}
}

func TestOpenRouterHTTPErrorIsUnknown(t *testing.T) {
	srv := tlsServer(t, func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusServiceUnavailable) })
	if _, err := (OpenRouterAPI{URL: srv.URL, Client: srv.Client()}).Discover(context.Background()); err == nil {
		t.Fatal("a non-200 status must be an error")
	}
}

func TestOpenRouterHarnessFieldOverridesDefault(t *testing.T) {
	if got := (OpenRouterAPI{HarnessID: "codex"}).Harness(); got != "codex" {
		t.Errorf("harness = %q", got)
	}
}

func TestOpenRouterIsOptIn(t *testing.T) {
	has := func() bool {
		for _, s := range Defaults() {
			if _, ok := s.(OpenRouterAPI); ok {
				return true
			}
		}
		return false
	}
	t.Setenv(OpenRouterEnv, "")
	if has() {
		t.Fatal("OpenRouter must not be a default source without the opt-in")
	}
	t.Setenv(OpenRouterEnv, "true")
	if has() {
		t.Fatal("only the exact value 1 opts in")
	}
	t.Setenv(OpenRouterEnv, "1")
	if !has() {
		t.Fatal("DOWNSHIFT_DISCOVERY_OPENROUTER=1 must add the source")
	}
}
