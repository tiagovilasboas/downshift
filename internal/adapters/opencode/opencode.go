// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package opencode adapts core routing decisions to OpenCode's
// subagent_spawn_intent hook protocol (wire v1).
//
// Route request, one JSON object on stdin:
//
//	{"v":1,"kind":"route","harness":{"id":"opencode","version":"2.0.32"},
//	 "event":"subagent_spawn_intent","correlation":{"child":"<hex>","parent":"<hex>"},
//	 "agent":"explore","model":{"providerID":"opencode","id":"big-pickle","variant":"default"},
//	 "taskText":"<transient classification input>","tsUtc":"…"}
//
// Route response, one JSON object on stdout:
//
//	{"v":1,"action":"allow"|"route","model":{"providerID":"opencode","id":"…"},
//	 "reasonCode":"routed|kept|held|unknown_session|fail_open","ttlMs":15000}
//
// taskText is a classification input and nothing else: this package never
// logs it, never echoes it, never persists it and never copies it into a
// response. The response carries only the allow-listed keys above. Unknown
// request fields are dropped by the strict decode; an invalid request is
// answered fail-open (allow, reasonCode "fail_open") with a zero Decision.
//
// Wording: this package reports what the hook decided — dispatch observed at
// the harness API boundary, model resolved to X (see port.go for the
// observation side). It never claims that a harness applied a rewrite.
//
// Caller contract (phase 2/3 — documented here, implemented outside this
// repo): the OpenCode plugin and the installer spawn this command by fixed
// absolute path with an argv array and never through a PATH lookup; the
// bridge pins a checksum next to that path. The plugin also owns its side of
// this wire: it must parse the response line strictly (allow-listed keys,
// size-capped) and treat anything else as allow. This package assumes those
// caller properties.
package opencode

import (
	"encoding/json"
	"regexp"

	"github.com/tiagovilasboas/downshift/internal/core"
)

const harnessID = "opencode"

// ttlMs is the cache lifetime the wire contract attaches to a routed or
// allowed answer; a fail-open answer omits it.
const ttlMs = 15000

// Field caps bound every request string before routing. A request that breaks
// a cap is not a valid route request: the hook fails open and records why.
// The shared hook runner already caps the whole stdin payload at 1 MiB; these
// caps are defense in depth for the classification path.
const (
	maxVersionLen     = 64
	maxCorrelationLen = 64
	maxAgentLen       = 64
	maxProviderIDLen  = 64
	maxModelIDLen     = 128
	maxVariantLen     = 128
	maxTaskTextBytes  = 64 << 10
	maxTsUtcLen       = 64
)

// childIDPattern is the correlation.child contract: lowercase hex, 16..64
// characters (the plugin sends an HMAC-derived id).
var childIDPattern = regexp.MustCompile(`^[a-f0-9]{16,64}$`)

// Request is one route request (wire v1). Unknown JSON fields are dropped by
// the strict decode below.
type Request struct {
	V           int             `json:"v"`
	Kind        string          `json:"kind"`
	Harness     RequestHarness  `json:"harness"`
	Event       string          `json:"event"`
	Correlation RequestRelation `json:"correlation"`
	Agent       string          `json:"agent"`
	Model       RequestModel    `json:"model"`
	TaskText    string          `json:"taskText"`
	TsUtc       string          `json:"tsUtc"`
}

// RequestHarness identifies which harness asked.
type RequestHarness struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

// RequestRelation is the spawn correlation: child identifies this subagent
// spawn, parent the session it was spawned from.
type RequestRelation struct {
	Child  string `json:"child"`
	Parent string `json:"parent"`
}

// RequestModel is the model the spawn would run on without a rewrite.
type RequestModel struct {
	ProviderID string `json:"providerID"`
	ID         string `json:"id"`
	Variant    string `json:"variant"`
}

// Response is the single JSON object answered on stdout. It carries only
// allow-listed keys; Model appears only on action "route" and TTLMs on every
// answer except a fail-open one.
type Response struct {
	V          int            `json:"v"`
	Action     string         `json:"action"`
	Model      *ResponseModel `json:"model,omitempty"`
	ReasonCode string         `json:"reasonCode"`
	TTLMs      int            `json:"ttlMs,omitempty"`
}

// ResponseModel is the model the plugin should run the child on.
type ResponseModel struct {
	ProviderID string `json:"providerID"`
	ID         string `json:"id"`
}

// Reason codes are stable, content-free names for why the hook answered the
// way it did.
const (
	reasonRouted         = "routed"
	reasonKept           = "kept"
	reasonHeld           = "held"
	reasonUnknownSession = "unknown_session"
	// ReasonFailOpen is exported for the CLI wire, which recognizes it to
	// record the fail-open outcome once.
	ReasonFailOpen = "fail_open"
)

// ParseRequest strictly decodes one route request. Unknown JSON fields are
// dropped; a payload that is not a JSON object with the expected field types
// is a decode error and the shared runner answers fail-open.
func ParseRequest(data []byte) (Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return Request{}, err
	}
	return req, nil
}

// SessionIdentifier returns the spawn correlation id the shared runner hashes
// for the decision's session field. It is the same id the observation side
// links by, so a decision and its later observations always meet.
func (r Request) SessionIdentifier() string { return r.Correlation.Child }

// ToolUseIdentifier returns the id of this spawn call, so a post-spawn record
// links to the exact decision made for it.
func (r Request) ToolUseIdentifier() string { return r.Correlation.Child }

// CorrelationIdentifier returns the wire correlation id; the shared runner
// uses it (when it matches its correlation contract) instead of generating
// one, so stderr and telemetry share one id per spawn.
func (r Request) CorrelationIdentifier() string { return r.Correlation.Child }

// RequestedModel returns the model the spawn asked for, never a routing
// recommendation.
func (r Request) RequestedModel() string { return r.Model.ID }

// validRequest reports whether the decoded request is a well-formed route
// request for this harness, within every field cap.
func validRequest(req Request) bool {
	return req.V == 1 &&
		req.Kind == "route" &&
		req.Harness.ID == harnessID &&
		req.Event == "subagent_spawn_intent" &&
		len(req.Harness.Version) <= maxVersionLen &&
		childIDPattern.MatchString(req.Correlation.Child) &&
		len(req.Correlation.Parent) <= maxCorrelationLen &&
		len(req.Agent) <= maxAgentLen &&
		len(req.Model.ProviderID) <= maxProviderIDLen &&
		req.Model.ID != "" && len(req.Model.ID) <= maxModelIDLen &&
		len(req.Model.Variant) <= maxVariantLen &&
		req.TaskText != "" && len(req.TaskText) <= maxTaskTextBytes &&
		len(req.TsUtc) <= maxTsUtcLen
}

// Handle processes one route request. An invalid request returns the
// fail-open response and a zero Decision: the data was not a subagent spawn
// question this hook can answer, so nothing is rewritten and no routing
// telemetry is recorded for it (the CLI wire records the fail-open outcome).
//
// A valid request routes through the same core as every harness: the session
// allowlist comes from the operator file (the wire carries no session lists),
// classification and guardrails are shared, and only the answer format is
// OpenCode's.
func Handle(req Request, r ...core.Resolver) (Response, string, core.Decision) {
	if !validRequest(req) {
		return failOpen(), "", core.Decision{}
	}
	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	child := req.Correlation.Child
	currentModel := req.Model.ID
	session := core.ResolveSessionForID(harnessID, child, nil, nil)
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.Route(req.TaskText, harnessID, currentModel, res)
	// The requested id is what the payload carried; provenance telemetry
	// compares it against the catalog allowlist instead of trusting it.
	decision.Baseline.RequestedID = currentModel

	plan := decision.PlanSession(core.OpenCodeCaps, res, session, currentModel)
	// A plan the guardrails or the credit gate held is answered "held": the
	// spawn is allowed unchanged and the recommendation stays in telemetry.
	held := plan.HoldForeign || plan.PreserveExplicit || plan.CreditHeld || len(decision.Corrections) > 0
	if plan.RewriteModel {
		if core.CanWriteSessionID(harnessID, plan.Model.ID, plan.Session, res) {
			decision.Model = plan.Model
			return routed(plan.Model), decision.Summary(), decision
		}
		held = true
	}
	switch {
	case held:
		return allow(reasonHeld), "", decision
	case decision.Session.Unknown:
		return allow(reasonUnknownSession), "", decision
	default:
		return allow(reasonKept), "", decision
	}
}

// routed is the answer that moves the child onto another model.
func routed(m core.Model) Response {
	provider := m.Harness
	if provider == "" {
		provider = harnessID
	}
	return Response{
		V:          1,
		Action:     "route",
		Model:      &ResponseModel{ProviderID: provider, ID: m.ID},
		ReasonCode: reasonRouted,
		TTLMs:      ttlMs,
	}
}

// allow is the answer that leaves the spawn on its requested model.
func allow(reason string) Response {
	return Response{V: 1, Action: "allow", ReasonCode: reason, TTLMs: ttlMs}
}

// failOpen is the response to anything this hook cannot answer; the shared
// runner's other failure paths print the same shape through the CLI wire.
func failOpen() Response {
	return Response{V: 1, Action: "allow", ReasonCode: ReasonFailOpen}
}
