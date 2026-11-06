// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func releaseServer(t *testing.T, status int, body string, hits *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hits, 1)
		w.WriteHeader(status)
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLatestRelease(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		status   int
		body     string
		wantTag  string
		wantOK   bool
		wantHits int32
	}{
		{"ok", 200, `{"tag_name":"v0.8.1"}`, "v0.8.1", true, 1},
		{"server error is unknown", 500, `oops`, "", false, 1},
		{"bad json is unknown", 200, `{`, "", false, 1},
		{"empty tag is unknown", 200, `{}`, "", false, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var hits int32
			srv := releaseServer(t, c.status, c.body, &hits)
			l := latestLookup{Client: srv.Client(), URL: srv.URL,
				CacheFile: filepath.Join(t.TempDir(), "latest.json"), Now: func() time.Time { return now }}
			tag, ok := l.Latest()
			if tag != c.wantTag || ok != c.wantOK || hits != c.wantHits {
				t.Fatalf("got %q,%v hits=%d", tag, ok, hits)
			}
		})
	}
}

func TestLatestReleaseCache(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	var hits int32
	srv := releaseServer(t, 200, `{"tag_name":"v0.8.1"}`, &hits)
	cache := filepath.Join(t.TempDir(), "sub", "latest.json")
	at := now
	l := latestLookup{Client: srv.Client(), URL: srv.URL, CacheFile: cache, Now: func() time.Time { return at }}

	if tag, ok := l.Latest(); !ok || tag != "v0.8.1" || hits != 1 {
		t.Fatalf("first: %q %v %d", tag, ok, hits)
	}
	at = now.Add(23 * time.Hour)
	if tag, ok := l.Latest(); !ok || tag != "v0.8.1" || hits != 1 {
		t.Fatalf("cache hit should not call the server: %q %v %d", tag, ok, hits)
	}
	at = now.Add(25 * time.Hour)
	if _, ok := l.Latest(); !ok || hits != 2 {
		t.Fatalf("expired cache should refetch: hits=%d", hits)
	}
}

func TestLatestReleaseOfflineKeepsNoVerdict(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // connection refused
	l := latestLookup{Client: &http.Client{Timeout: time.Second}, URL: url,
		CacheFile: filepath.Join(t.TempDir(), "latest.json"), Now: time.Now}
	if tag, ok := l.Latest(); ok || tag != "" {
		t.Fatalf("offline must be unknown, got %q %v", tag, ok)
	}
}
