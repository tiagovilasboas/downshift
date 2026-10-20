// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package openrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/gateway"
	"github.com/tiagovilasboas/downshift/internal/gateway/gatewaytest"
)

func fixedSnapshot(model string) (float64, float64, bool) {
	if model == "vendor/priced" {
		return 0.5, 1.5, true
	}
	return 0, 0, false
}

func TestContract(t *testing.T) {
	gatewaytest.Check(t, New(fixedSnapshot), []string{"vendor/priced", "x-ai/grok-x"})
	gatewaytest.Check(t, New(nil), []string{"vendor/model"})
}

func TestNameIsRegistryKey(t *testing.T) {
	if got := New(nil).Name(); got != "openrouter" {
		t.Fatalf("Name() = %q, want openrouter", got)
	}
}

func TestPriceWithoutSnapshotIsUnknown(t *testing.T) {
	if _, _, ok := New(nil).Price("vendor/priced"); ok {
		t.Fatal("a gateway with no snapshot must price nothing")
	}
	if in, out, ok := New(fixedSnapshot).Price("vendor/priced"); !ok || in != 0.5 || out != 1.5 {
		t.Fatalf("Price from snapshot = %v/%v/%v", in, out, ok)
	}
}

func TestPrivacyDefaultIsUnknown(t *testing.T) {
	got := New(nil).Privacy("vendor/model")
	if got.Retention != gateway.RetentionUnknown {
		t.Fatalf("Retention = %q, want unknown by default", got.Retention)
	}
}

func TestPerMillionRejectsNaNAndInfinity(t *testing.T) {
	for _, s := range []string{"", "NaN", "Inf", "-Inf", "-0.5", "abc"} {
		if perMillion(s) != nil {
			t.Errorf("perMillion(%q) must be nil", s)
		}
	}
}

func TestPerMillionConverts(t *testing.T) {
	got := perMillion("0.000002")
	if got == nil || *got < 1.999999 || *got > 2.000001 {
		t.Fatalf("perMillion(0.000002) = %v, want 2", got)
	}
}

func TestOversizeBodyIsAnError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(strings.Repeat("a", maxBody+1)))
	}))
	t.Cleanup(srv.Close)
	if _, err := (Gateway{URL: srv.URL, Client: srv.Client()}).ListModels(context.Background()); err == nil {
		t.Fatal("an oversize body must be an error, not a truncated list")
	}
}

func TestParseListSkipsUnpricedAndKeepsIDs(t *testing.T) {
	data := []byte(`{"data":[
		{"id":"vendor/a","name":"A","created":1,"pricing":{"prompt":"0.000001","completion":"0.000002"}},
		{"id":"vendor/b","pricing":{"prompt":"-1","completion":"x"}},
		{"id":"","pricing":{}}]}`)
	ms, err := parseList(data)
	if err != nil {
		t.Fatalf("parseList: %v", err)
	}
	if len(ms) != 2 || ms[0].ID != "vendor/a" || ms[1].ID != "vendor/b" {
		t.Fatalf("models = %+v", ms)
	}
	if ms[1].InputM != nil || ms[1].OutputM != nil {
		t.Fatal("dynamic or non-numeric prices must be unknown")
	}
}
