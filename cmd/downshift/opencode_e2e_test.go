// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

// OpenCode route wire end-to-end tests: the registered hook (shared runner +
// openCodeWire) through stdout and telemetry, including both fail-open paths
// (malformed payload, oversized payload, request over a field cap), the
// no-route envelope, and the honor observation linking the decision it
// observed. Fixtures are synthetic: placeholder task text, fixed hex
// correlation ids, catalog model ids.

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/core/decidertest"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

const openCodeChild = "5ab1c2d3e4f5a6b7c8d9e0f1a2b3c4d5"

// openCodeEnv isolates one hook run and returns the event log path.
func openCodeEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DOWNSHIFT_STATE_DIR", t.TempDir())
	log := filepath.Join(t.TempDir(), "events.jsonl")
	t.Setenv("DOWNSHIFT_EVENT_LOG", log)
	t.Setenv("DOWNSHIFT_CONTROL_SHARE", "0")
	return log
}

// runOpenCodeHook drives the registered opencode hook on one stdin payload.
func runOpenCodeHook(t *testing.T, stdin string) (rc int, stdout, stderr string) {
	t.Helper()
	stdout = captureStdout(func() {
		stderr = captureStderr(func() {
			rc = harnessRegistry["opencode"].Hook(strings.NewReader(stdin), cmdCat)
		})
	})
	return rc, stdout, stderr
}

// openCodeRequest marshals one valid wire v1 route request.
func openCodeRequest(t *testing.T, taskText, model string) string {
	t.Helper()
	req := opencode.Request{
		V:           1,
		Kind:        "route",
		Harness:     opencode.RequestHarness{ID: "opencode", Version: "2.0.32"},
		Event:       "subagent_spawn_intent",
		Correlation: opencode.RequestRelation{Child: openCodeChild, Parent: "0f1e2d3c4b5a69788796a5b4c3d2e1f0"},
		Agent:       "general",
		Model:       opencode.RequestModel{ProviderID: "opencode", ID: model, Variant: "default"},
		TaskText:    taskText,
		TsUtc:       "2026-10-18T12:00:00Z",
	}
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// openCodeAnswer is the decoded route response.
type openCodeAnswer struct {
	V          int    `json:"v"`
	Action     string `json:"action"`
	ReasonCode string `json:"reasonCode"`
	TTLMs      int    `json:"ttlMs"`
	Model      *struct {
		ProviderID string `json:"providerID"`
		ID         string `json:"id"`
	} `json:"model"`
}

// decodeAnswer decodes stdout once typed and once as raw keys, so tests can
// assert both the values and which keys the wire actually emitted.
func decodeAnswer(t *testing.T, stdout string) (openCodeAnswer, map[string]any) {
	t.Helper()
	var ans openCodeAnswer
	trimmed := strings.TrimSpace(stdout)
	if err := json.Unmarshal([]byte(trimmed), &ans); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
	}
	var keys map[string]any
	if err := json.Unmarshal([]byte(trimmed), &keys); err != nil {
		t.Fatal(err)
	}
	return ans, keys
}

func openCodeTierIDs(t *testing.T) (small, mid, frontier string) {
	t.Helper()
	small = cmdCat.ModelFor("opencode", core.TierSmall).ID
	mid = cmdCat.ModelFor("opencode", core.TierMid).ID
	frontier = cmdCat.ModelFor("opencode", core.TierFrontier).ID
	if small == "" || mid == "" || frontier == "" {
		t.Fatalf("catalog lacks an opencode tier model (small=%q mid=%q frontier=%q)", small, mid, frontier)
	}
	return small, mid, frontier
}

func TestOpenCodeE2E_RouteEnvelopeCarriesOnlyAllowListedKeys(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	log := openCodeEnv(t)
	small, mid, frontier := openCodeTierIDs(t)
	setSessionAllowlist(t, "opencode", []string{small, mid, frontier})
	rc, stdout, stderr := runOpenCodeHook(t, openCodeRequest(t, "rename the userId variable to userIdentifier", frontier))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	ans, keys := decodeAnswer(t, stdout)
	if ans.V != 1 || ans.Action != "route" || ans.ReasonCode != "routed" || ans.TTLMs != 15000 {
		t.Fatalf("answer = %+v", ans)
	}
	if ans.Model == nil || ans.Model.ID != small || ans.Model.ProviderID != "opencode" {
		t.Fatalf("routed model = %+v, want %s", ans.Model, small)
	}
	if len(keys) != 5 {
		t.Fatalf("route envelope keys = %v, want exactly v/action/model/reasonCode/ttlMs", keys)
	}
	if !strings.Contains(stderr, "honor=unobserved") {
		t.Errorf("spawn line must state honor is unobserved: %q", stderr)
	}
	events, err := telemetry.ReadEventsFrom(log)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeRewriteEmitted ||
		events[0].ToModel != small || events[0].Harness != "opencode" {
		t.Fatalf("decision event = %+v", events)
	}
}

func TestOpenCodeE2E_MalformedStdinFailsOpenAndIsRecorded(t *testing.T) {
	log := openCodeEnv(t)
	rc, stdout, _ := runOpenCodeHook(t, `{"v":1,"kind":"route",`)
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	ans, keys := decodeAnswer(t, stdout)
	if ans.Action != "allow" || ans.ReasonCode != "fail_open" || ans.Model != nil {
		t.Fatalf("answer = %+v", ans)
	}
	if _, ok := keys["ttlMs"]; ok {
		t.Fatalf("a fail-open answer must omit ttlMs: %v", keys)
	}
	events, err := telemetry.ReadEventsFrom(log)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want the failure plus the fail-open record", len(events))
	}
	if events[0].Outcome != "error" || events[0].ErrorCode != "INVALID_EVENT" {
		t.Fatalf("first record = %+v, want the INVALID_EVENT failure", events[0])
	}
	if events[1].Outcome != telemetry.OutcomeFailOpen || events[1].ErrorCode != "INVALID_EVENT" || events[1].Harness != "opencode" {
		t.Fatalf("second record = %+v, want the INVALID_EVENT fail-open outcome", events[1])
	}
}

func TestOpenCodeE2E_OversizedStdinFailsOpenAndIsRecorded(t *testing.T) {
	log := openCodeEnv(t)
	rc, stdout, _ := runOpenCodeHook(t, strings.Repeat("a", (1<<20)+2))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	ans, _ := decodeAnswer(t, stdout)
	if ans.Action != "allow" || ans.ReasonCode != "fail_open" {
		t.Fatalf("answer = %+v", ans)
	}
	events, err := telemetry.ReadEventsFrom(log)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].ErrorCode != "PAYLOAD_TOO_LARGE" ||
		events[1].Outcome != telemetry.OutcomeFailOpen || events[1].ErrorCode != "PAYLOAD_TOO_LARGE" {
		t.Fatalf("events = %+v", events)
	}
}

func TestOpenCodeE2E_TaskOverCapFailsOpenOnce(t *testing.T) {
	log := openCodeEnv(t)
	setSessionAllowlist(t, "opencode", []string{"big-pickle"})
	rc, stdout, _ := runOpenCodeHook(t, openCodeRequest(t, strings.Repeat("t", 64<<10+1), "big-pickle"))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	ans, keys := decodeAnswer(t, stdout)
	if ans.Action != "allow" || ans.ReasonCode != "fail_open" {
		t.Fatalf("answer = %+v", ans)
	}
	if _, ok := keys["ttlMs"]; ok {
		t.Fatalf("a fail-open answer must omit ttlMs: %v", keys)
	}
	events, err := telemetry.ReadEventsFrom(log)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeFailOpen || events[0].ErrorCode != "INVALID_REQUEST" {
		t.Fatalf("events = %+v, want one INVALID_REQUEST fail-open record", events)
	}
}

func TestOpenCodeE2E_NoRouteAnswersTheCacheableEnvelope(t *testing.T) {
	log := openCodeEnv(t)
	small, mid, frontier := openCodeTierIDs(t)
	setSessionAllowlist(t, "opencode", []string{small, mid, frontier})
	t.Setenv("DOWNSHIFT_NO_ROUTE", "1")
	rc, stdout, _ := runOpenCodeHook(t, openCodeRequest(t, "rename the userId variable to userIdentifier", frontier))
	if rc != 0 {
		t.Fatalf("rc = %d", rc)
	}
	ans, keys := decodeAnswer(t, stdout)
	if ans.Action != "allow" || ans.ReasonCode != "no_route" || ans.TTLMs != 15000 {
		t.Fatalf("answer = %+v", ans)
	}
	if len(keys) != 4 {
		t.Fatalf("no_route envelope keys = %v", keys)
	}
	events, err := telemetry.ReadEventsFrom(log)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Outcome != telemetry.OutcomeBaseline {
		t.Fatalf("events = %+v, want one baseline decision", events)
	}
	for _, ev := range events {
		if ev.Outcome == telemetry.OutcomeFailOpen {
			t.Fatalf("no_route is not a failure: %+v", ev)
		}
	}
}

func TestOpenCodeE2E_HonorObservationLinksTheRoutedDecision(t *testing.T) {
	decidertest.Use(t, decidertest.Tier(core.TierSmall))
	log := openCodeEnv(t)
	small, mid, frontier := openCodeTierIDs(t)
	setSessionAllowlist(t, "opencode", []string{small, mid, frontier})
	if rc, _, _ := runOpenCodeHook(t, openCodeRequest(t, "rename the userId variable to userIdentifier", frontier)); rc != 0 {
		t.Fatalf("route rc = %d", rc)
	}
	obs := strings.Join([]string{
		`{"v":1,"kind":"observation","correlation":{"child":"` + openCodeChild + `"},"event":"switch_attempt","result":"ok"}`,
		`{"v":1,"kind":"observation","correlation":{"child":"` + openCodeChild + `"},"event":"model_selected","model":"` + small + `","previous":"` + frontier + `"}`,
		`{"v":1,"kind":"observation","correlation":{"child":"` + openCodeChild + `"},"event":"child_context","model":"` + small + `"}`,
	}, "\n") + "\n"
	var rc int
	stdout := captureStdout(func() {
		rc = runHonor(strings.NewReader(obs), *harnessRegistry["opencode"].Port, cmdCat)
	})
	if rc != 0 || strings.TrimSpace(stdout) != "{}" {
		t.Fatalf("honor rc=%d stdout=%q", rc, stdout)
	}
	events, err := telemetry.ReadEventsFrom(log)
	if err != nil {
		t.Fatal(err)
	}
	var resolved *telemetry.Event
	for i := range events {
		if events[i].Outcome == telemetry.OutcomeResolved {
			resolved = &events[i]
		}
	}
	if resolved == nil {
		t.Fatalf("no resolved record: %+v", events)
	}
	if resolved.RewriteHonored == nil || !*resolved.RewriteHonored || resolved.ToModel != small {
		t.Fatalf("resolution not confirmed against the decision target: %+v", resolved)
	}
}
