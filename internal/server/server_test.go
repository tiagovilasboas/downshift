// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: BUSL-1.1
// Commercial use requires a licence — see LICENSE for terms.

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLocalOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // no events: handlers return empty data
	h := newHandler("")

	cases := []struct {
		name, path, host, origin string
		want                     int
	}{
		{"loopback ip", "/api/status", "127.0.0.1:7474", "", http.StatusOK},
		{"localhost", "/health", "localhost:7474", "", http.StatusOK},
		{"ipv6 loopback", "/health", "[::1]:7474", "", http.StatusOK},
		{"same-origin page", "/api/status", "127.0.0.1:7474", "http://127.0.0.1:7474", http.StatusOK},
		{"dns rebinding host", "/api/status", "evil.example:7474", "", http.StatusForbidden},
		{"foreign origin", "/api/status", "127.0.0.1:7474", "https://evil.example", http.StatusForbidden},
		{"foreign origin health", "/health", "localhost:7474", "https://evil.example", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("status = %d, want %d", rec.Code, tc.want)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
				t.Errorf("unexpected Access-Control-Allow-Origin %q", got)
			}
		})
	}
}
