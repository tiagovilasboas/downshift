// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package omniroute_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway/omniroute"
)

func TestOmniRouteNormalizeID(t *testing.T) {
	a := omniroute.New(time.Second)

	cases := []struct {
		raw   string
		valid bool
		want  string
	}{
		{"cc/claude-3-5-sonnet", true, "cc/claude-3-5-sonnet"},
		{"openai/gpt-4o", true, "openai/gpt-4o"},
		{"deepseek/deepseek-chat", true, "deepseek/deepseek-chat"},
		{"auto/coding", true, "auto/coding"},
		{"claude-3-5-sonnet", false, ""},
		{"a/b/c", false, ""},
		{"/model", false, ""},
		{"provider/", false, ""},
		{"", false, ""},
	}

	for _, tc := range cases {
		got, ok := a.NormalizeID(tc.raw)
		if ok != tc.valid {
			t.Errorf("NormalizeID(%q) ok = %v, want %v", tc.raw, ok, tc.valid)
		}
		if ok && got != tc.want {
			t.Errorf("NormalizeID(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

func TestOmniRouteCheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" || r.URL.Path == "/models" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	a := omniroute.New(2 * time.Second)
	ok, msg := a.CheckHealth(context.Background(), ts.URL)
	if !ok {
		t.Fatalf("expected healthcheck ok, got false, msg: %s", msg)
	}
}
