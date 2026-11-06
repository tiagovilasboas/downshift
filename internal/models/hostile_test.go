// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package models_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/models"
)

// Anthropic's key travels in x-api-key, which Go does not strip on a redirect.
// The listing must stop at the redirect: the second host never sees the key.
func TestCheck_RedirectDoesNotForwardTheAPIKey(t *testing.T) {
	var leaked atomic.Int32
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "" || r.Header.Get("Authorization") != "" {
			leaked.Add(1)
		}
		w.Write([]byte(anthropicResponse("claude-evil-1")))
	}))
	defer evil.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL, http.StatusFound)
	}))
	defer origin.Close()

	t.Setenv("ANTHROPIC_API_KEY", "sk-secret")
	t.Setenv("OPENAI_API_KEY", "")
	var out, errOut bytes.Buffer
	models.CheckWithProviders(cat, mockProviders(origin.URL, "http://unused"), &http.Client{}, &out, &errOut)

	if leaked.Load() != 0 {
		t.Fatal("the API key was forwarded to the redirect target")
	}
	if strings.Contains(out.String(), "claude-evil-1") {
		t.Errorf("a model list from the redirect target was used:\n%s", out.String())
	}
	if !strings.Contains(errOut.String(), "HTTP 302") {
		t.Errorf("the refused redirect should be reported as an error; stderr: %q", errOut.String())
	}
}

// Ids with escape sequences, spaces or absurd length are dropped before they
// are printed or stored.
func TestCheck_DropsHostileModelIDs(t *testing.T) {
	srv := serveModels(t, openAIResponse(
		"gpt-new-1",
		"evil\x1b[2Jid",
		"has space",
		"-leading-dash",
		strings.Repeat("a", 300),
		"semi;colon",
	))
	defer srv.Close()
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("ANTHROPIC_API_KEY", "")
	var out, errOut bytes.Buffer
	models.CheckWithProviders(cat, mockProviders("http://unused", srv.URL), &http.Client{}, &out, &errOut)
	text := out.String()
	if !strings.Contains(text, "gpt-new-1") {
		t.Errorf("a valid new id was lost:\n%s", text)
	}
	for _, bad := range []string{"\x1b", "has space", "-leading-dash", "semi;colon", strings.Repeat("a", 300)} {
		if strings.Contains(text, bad) {
			t.Errorf("hostile id %q reached the output:\n%s", bad, text)
		}
	}
}

// The body is read through a cap: an endless answer ends in an error, not in
// memory growth.
func TestCheck_CapsTheResponseBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[`))
		chunk := []byte(strings.Repeat(`{"id":"x"},`, 1000))
		for i := 0; i < 2000; i++ { // ~22 MB, past the 4 MiB cap
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	t.Setenv("OPENAI_API_KEY", "k")
	t.Setenv("ANTHROPIC_API_KEY", "")
	var out, errOut bytes.Buffer
	models.CheckWithProviders(cat, mockProviders("http://unused", srv.URL), &http.Client{}, &out, &errOut)
	if !strings.Contains(errOut.String(), "parse response") {
		t.Errorf("an oversized body should fail to parse, stderr: %q stdout: %q", errOut.String(), out.String())
	}
}
