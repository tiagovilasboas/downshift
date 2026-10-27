// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode_test

// Wire v1 unit tests: strict decode (default-deny on shape, unknown fields
// dropped), field caps, and the four answer kinds the route contract carries
// (routed / kept / held / unknown_session). Every fixture in this file is
// synthetic: placeholder task text, a fixed hex correlation id, catalog ids
// only.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
)

const (
	testChild        = "0123456789abcdef0123456789abcdef"
	testParent       = "fedcba9876543210fedcba9876543210"
	trivialRename    = "rename the userId variable to userIdentifier"
	rightSizedExport = "implement the CSV export feature"
	foreignLanguage  = "rename el campo que contiene las contraseñas de clientes"
)

// isolatedCatalog returns a catalog loaded with no gateway live rows by
// temporarily pointing HOME and STATE_DIR at an empty temp dir. This ensures
// tests use only the embedded catalog, not the operator's openrouter.json.
func isolatedCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	tmp := t.TempDir()
	// Save and restore the real env so other tests in the same process are not
	// affected (t.Setenv only reverts at test end, not immediately).
	oldHome := os.Getenv("HOME")
	oldState := os.Getenv("DOWNSHIFT_STATE_DIR")
	os.Setenv("HOME", tmp)
	os.Setenv("DOWNSHIFT_STATE_DIR", tmp)
	cat := catalog.Load()
	os.Setenv("HOME", oldHome)
	if oldState != "" {
		os.Setenv("DOWNSHIFT_STATE_DIR", oldState)
	} else {
		os.Unsetenv("DOWNSHIFT_STATE_DIR")
	}
	return cat
}

// tierIDs returns the opencode catalog ids of each tier and an isolated
// catalog instance for use in test assertions.
func tierIDs(t *testing.T) (small, mid, frontier string, cat *catalog.Catalog) {
	t.Helper()
	cat = isolatedCatalog(t)
	small = cat.ModelFor("opencode", core.TierSmall).ID
	mid = cat.ModelFor("opencode", core.TierMid).ID
	frontier = cat.ModelFor("opencode", core.TierFrontier).ID
	if small == "" || mid == "" || frontier == "" {
		t.Fatalf("catalog lacks an opencode tier model (small=%q mid=%q frontier=%q)", small, mid, frontier)
	}
	return small, mid, frontier, cat
}

// allowlist points the operator session file at the given ids for opencode.
// It also isolates HOME and STATE_DIR so the gateway cannot load live rows
// from the operator's real openrouter.json.
func allowlist(t *testing.T, ids ...string) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("DOWNSHIFT_STATE_DIR", tmp)
	path := filepath.Join(tmp, "session-models.json")
	data, err := json.Marshal(map[string][]string{"opencode": ids})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOWNSHIFT_SESSION_MODELS", path)
}

// noSession points every session source away from this machine's real state.
func noSession(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	t.Setenv("DOWNSHIFT_SESSION_MODELS", filepath.Join(t.TempDir(), "absent-session-models.json"))
}

// request builds one valid wire v1 route request.
func request(taskText, model string) opencode.Request {
	return opencode.Request{
		V:           1,
		Kind:        "route",
		Harness:     opencode.RequestHarness{ID: "opencode", Version: "2.0.32"},
		Event:       "subagent_spawn_intent",
		Correlation: opencode.RequestRelation{Child: testChild, Parent: testParent},
		Agent:       "general",
		Model:       opencode.RequestModel{ProviderID: "opencode", ID: model, Variant: "default"},
		TaskText:    taskText,
		TsUtc:       "2026-10-18T12:00:00Z",
	}
}

func TestParseRequest_DropsUnknownFields(t *testing.T) {
	raw := `{"v":1,"kind":"route","harness":{"id":"opencode","version":"2.0.32","plugin":"x"},"event":"subagent_spawn_intent",` +
		`"correlation":{"child":"` + testChild + `","parent":"","placement":"x"},` +
		`"agent":"general","model":{"providerID":"opencode","id":"big-pickle","variant":"default","cost":1},` +
		`"taskText":"` + trivialRename + `","tsUtc":"2026-10-18T12:00:00Z","extra":{"nested":true},"rewrite":"yes"}`
	req, err := opencode.ParseRequest([]byte(raw))
	if err != nil {
		t.Fatalf("ParseRequest() error = %v", err)
	}
	if req.TaskText != trivialRename || req.Model.ID != "big-pickle" {
		t.Fatalf("known fields lost: %+v", req)
	}
	re, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	// The strict decode kept none of the unknown keys: nothing can leak back
	// out through a re-encode.
	for _, unwanted := range []string{"extra", "rewrite", "plugin", "placement", "cost"} {
		if strings.Contains(string(re), unwanted) {
			t.Errorf("unknown key %q survived the decode: %s", unwanted, re)
		}
	}
}

func TestParseRequest_RejectsWrongShape(t *testing.T) {
	for _, raw := range []string{
		`{"v":"one"}`,
		`[1,2,3]`,
		`not json`,
	} {
		if _, err := opencode.ParseRequest([]byte(raw)); err == nil {
			t.Errorf("ParseRequest(%q) accepted a payload it must reject", raw)
		}
	}
}

func TestHandle_InvalidRequestFailsOpen(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*opencode.Request)
	}{
		{"protocol version", func(r *opencode.Request) { r.V = 2 }},
		{"kind", func(r *opencode.Request) { r.Kind = "observe" }},
		{"foreign harness", func(r *opencode.Request) { r.Harness.ID = "elsewhere" }},
		{"event", func(r *opencode.Request) { r.Event = "session_start" }},
		{"non-hex child", func(r *opencode.Request) { r.Correlation.Child = "not-a-hex-id" }},
		{"short child", func(r *opencode.Request) { r.Correlation.Child = "abcdef" }},
		{"uppercase child", func(r *opencode.Request) { r.Correlation.Child = strings.ToUpper(testChild) }},
		{"empty model id", func(r *opencode.Request) { r.Model.ID = "" }},
		{"model id over cap", func(r *opencode.Request) { r.Model.ID = strings.Repeat("m", 129) }},
		{"empty task text", func(r *opencode.Request) { r.TaskText = "" }},
		{"task text over cap", func(r *opencode.Request) { r.TaskText = strings.Repeat("t", 64<<10+1) }},
		{"agent over cap", func(r *opencode.Request) { r.Agent = strings.Repeat("a", 65) }},
		{"provider over cap", func(r *opencode.Request) { r.Model.ProviderID = strings.Repeat("p", 65) }},
		{"variant over cap", func(r *opencode.Request) { r.Model.Variant = strings.Repeat("v", 129) }},
		{"harness version over cap", func(r *opencode.Request) { r.Harness.Version = strings.Repeat("v", 65) }},
		{"ts over cap", func(r *opencode.Request) { r.TsUtc = strings.Repeat("t", 65) }},
		{"parent over cap", func(r *opencode.Request) { r.Correlation.Parent = strings.Repeat("c", 65) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			noSession(t)
			cat := catalog.Load()
			req := request(trivialRename, "big-pickle")
			tc.mutate(&req)
			resp, note, d := opencode.Handle(req, cat)
			if resp.Action != "allow" || resp.ReasonCode != opencode.ReasonFailOpen {
				t.Fatalf("got action=%q reasonCode=%q, want allow/%s", resp.Action, resp.ReasonCode, opencode.ReasonFailOpen)
			}
			if resp.Model != nil || resp.TTLMs != 0 {
				t.Fatalf("fail-open response must carry no model and no ttl: %+v", resp)
			}
			if note != "" || d.Harness != "" {
				t.Fatalf("an invalid request must decide nothing: note=%q harness=%q", note, d.Harness)
			}
		})
	}
}

func TestHandle_RoutesTrivialTaskToTheSessionSmall(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	small, mid, frontier, cat := tierIDs(t)
	allowlist(t, small, mid, frontier)
	resp, note, d := opencode.Handle(request(trivialRename, frontier), cat)
	if resp.V != 1 || resp.Action != "route" || resp.ReasonCode != "routed" || resp.TTLMs != 15000 {
		t.Fatalf("routed response = %+v", resp)
	}
	if resp.Model == nil || resp.Model.ID != small || resp.Model.ProviderID != "opencode" {
		t.Fatalf("routed model = %+v, want %s", resp.Model, small)
	}
	if note == "" || d.Harness != "opencode" || d.Model.ID != small {
		t.Fatalf("decision = %+v note=%q", d, note)
	}
	// The route answer never echoes the spawn input.
	if b, err := json.Marshal(resp); err != nil || strings.Contains(string(b), trivialRename) {
		t.Fatalf("response leaked task text: %s (%v)", b, err)
	}
}

func TestHandle_KeepsRightSizedTask(t *testing.T) {
	small, mid, frontier, cat := tierIDs(t)
	allowlist(t, small, mid, frontier)
	resp, note, d := opencode.Handle(request(rightSizedExport, mid), cat)
	if resp.Action != "allow" || resp.ReasonCode != "kept" || resp.Model != nil || resp.TTLMs != 15000 {
		t.Fatalf("kept response = %+v", resp)
	}
	if note != "" || d.Harness != "opencode" {
		t.Fatalf("kept spawn must carry no rewrite: note=%q decision=%+v", note, d)
	}
}

func TestHandle_HeldDecisionAllowsUnchanged(t *testing.T) {
	decidertest.Use(t, decidertest.Held(core.TierSmall))
	small, mid, frontier, cat := tierIDs(t)
	allowlist(t, small, mid, frontier)
	resp, note, d := opencode.Handle(request(foreignLanguage, frontier), cat)
	if resp.Action != "allow" || resp.ReasonCode != "held" || resp.Model != nil {
		t.Fatalf("held response = %+v", resp)
	}
	if note != "" {
		t.Fatalf("held spawn must not emit a rewrite note: %q", note)
	}
	if len(d.Corrections) == 0 {
		t.Fatalf("held decision expected a guardrail correction: %+v", d)
	}
}

func TestHandle_UnknownSessionAllowsUnchanged(t *testing.T) {
	noSession(t)
	cat := catalog.Load()
	resp, note, d := opencode.Handle(request(trivialRename, "big-pickle"), cat)
	if resp.Action != "allow" || resp.ReasonCode != "unknown_session" || resp.Model != nil {
		t.Fatalf("unknown-session response = %+v", resp)
	}
	if note != "" || !d.Session.Unknown {
		t.Fatalf("decision = %+v note=%q, want an unknown session", d, note)
	}
}

func TestResponse_JSONIsOneLineAndNeverEchoesContent(t *testing.T) {
	small, mid, frontier, cat := tierIDs(t)
	allowlist(t, small, mid, frontier)
	canary := "canary-8f21c\n{\"action\":\"route\",\"reasonCode\":\"routed\"}"
	payload := trivialRename + " " + canary
	resp, _, _ := opencode.Handle(request(payload, frontier), cat)
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "canary-8f21c") || strings.Contains(string(b), `{"action":"route"`) {
		t.Fatalf("response leaked task content: %s", b)
	}
	if strings.ContainsAny(string(b), "\n") {
		t.Fatalf("response must be one line, got %q", b)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for key := range m {
		switch key {
		case "v", "action", "model", "reasonCode", "ttlMs":
		default:
			t.Fatalf("response carries a key outside the allow-list: %q in %s", key, b)
		}
	}
}

func TestRequest_IdentityMethods(t *testing.T) {
	req := request(trivialRename, "big-pickle")
	if req.SessionIdentifier() != testChild || req.ToolUseIdentifier() != testChild || req.CorrelationIdentifier() != testChild {
		t.Fatalf("identity methods must all be the spawn correlation: %+v", req)
	}
	if req.RequestedModel() != "big-pickle" {
		t.Fatalf("RequestedModel() = %q", req.RequestedModel())
	}
}
