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
	"time"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:7474"+path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestServerBoundsSlowClients(t *testing.T) {
	s := newServer("127.0.0.1:0", http.NotFoundHandler())
	if s.ReadHeaderTimeout <= 0 || s.ReadTimeout <= 0 || s.WriteTimeout <= 0 || s.IdleTimeout <= 0 {
		t.Fatalf("a timeout is unset: %+v", s)
	}
	if s.ReadHeaderTimeout > 10*time.Second {
		t.Errorf("ReadHeaderTimeout = %v, want a few seconds", s.ReadHeaderTimeout)
	}
}

// The embedded dashboard is what `downshift serve` serves by default.
func TestEmbeddedDashboardIsServed(t *testing.T) {
	t.Setenv(WebDirEnv, "")
	h := newHandlerFS(WebRoot())
	rec := get(h, "/")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("/ = %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(h, "/app.js"); rec.Code != 200 {
		t.Fatalf("/app.js = %d", rec.Code)
	}
	if rec := get(h, "/app.test.cjs"); rec.Code != 404 {
		t.Fatalf("test files must not be embedded: /app.test.cjs = %d", rec.Code)
	}
}

func TestDirectoriesAreNotListed(t *testing.T) {
	t.Setenv(WebDirEnv, "")
	h := newHandlerFS(WebRoot())
	for _, p := range []string{"/brand/", "/brand"} {
		rec := get(h, p)
		if rec.Code == 200 || strings.Contains(rec.Body.String(), "downshift-brand-tokens.css") {
			t.Errorf("%s listed the directory: %d %q", p, rec.Code, rec.Body.String())
		}
	}
	if rec := get(h, "/brand/downshift-brand-tokens.css"); rec.Code != 200 {
		t.Errorf("a file inside the directory must still be served: %d", rec.Code)
	}
}

// Only the embed or an explicit directory: never ./web of the working directory.
func TestWebRootIgnoresTheWorkingDirectory(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cwd, "web", "index.html"), []byte("<html>PLANTED</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(cwd); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)
	t.Setenv(WebDirEnv, "")
	if rec := get(newHandlerFS(WebRoot()), "/"); strings.Contains(rec.Body.String(), "PLANTED") {
		t.Fatal("served ./web from the working directory")
	}

	explicit := t.TempDir()
	os.WriteFile(filepath.Join(explicit, "index.html"), []byte("<html>EXPLICIT</html>"), 0o644)
	t.Setenv(WebDirEnv, explicit)
	if rec := get(newHandlerFS(WebRoot()), "/"); !strings.Contains(rec.Body.String(), "EXPLICIT") {
		t.Fatalf("explicit directory not served: %q", rec.Body.String())
	}
}
