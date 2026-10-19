// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The dashboard answers with a CSP header (frame-ancestors only works as a
// header) on the page, the API and the event stream route alike.
func TestResponsesCarryTheCSPHeader(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	web := t.TempDir()
	if err := os.WriteFile(filepath.Join(web, "index.html"), []byte("<html></html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := newHandler(web)
	for _, path := range []string{"/", "/health", "/api/status"} {
		req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7474"+path, nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		csp := rec.Header().Get("Content-Security-Policy")
		for _, want := range []string{"default-src 'self'", "script-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
			if !strings.Contains(csp, want) {
				t.Errorf("%s: CSP %q lacks %q", path, csp, want)
			}
		}
		if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("%s: missing nosniff", path)
		}
	}
}

// A rejected request (foreign host) is refused before any page is served.
func TestForeignHostStillRefused(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://evil.example/health", nil)
	rec := httptest.NewRecorder()
	newHandler("").ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("code = %d, want 403", rec.Code)
	}
}

// The header must not be weaker than the meta tag in web/index.html.
func TestHeaderCSPMatchesTheMetaTag(t *testing.T) {
	b, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Skip(err)
	}
	for _, directive := range strings.Split(dashboardCSP, "; ") {
		if strings.HasPrefix(directive, "frame-ancestors") {
			continue // not allowed in a meta tag
		}
		if !strings.Contains(string(b), directive) {
			t.Errorf("web/index.html meta CSP lacks %q", directive)
		}
	}
}
