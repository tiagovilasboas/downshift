// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package ninerouter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tiagovilasboas/downshift/internal/gateway/ninerouter"
)

func TestNineRouterNormalizeID(t *testing.T) {
	a := ninerouter.New(time.Second)

	cases := []struct {
		raw   string
		valid bool
		want  string
	}{
		{"anthropic/claude-3-5-sonnet", true, "anthropic/claude-3-5-sonnet"},
		{"deepseek/deepseek-chat", true, "deepseek/deepseek-chat"},
		{"claude-3-5-sonnet", true, "claude-3-5-sonnet"},
		{"a/b/c", false, ""},
		{"/model", false, ""},
		{"model/", false, ""},
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

func TestNineRouterCheckHealth(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer ts.Close()

	a := ninerouter.New(2 * time.Second)
	ok, msg := a.CheckHealth(context.Background(), ts.URL)
	if !ok {
		t.Fatalf("expected healthcheck ok, got false, msg: %s", msg)
	}
}
