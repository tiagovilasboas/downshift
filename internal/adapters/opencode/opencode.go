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
// Caller contract: the bridge (hooks/opencode/index.ts, installed by
// `downshift setup` into OpenCode's plugins folder) runs this command with an
// argv array and no shell, through DOWNSHIFT_BIN or `downshift` on PATH, with
// a per-call timeout and a size cap. It reads the first non-empty response
// line and uses only the fields above; any other output, a
// non-zero exit or a timeout leaves the spawn unchanged. Pinning the binary by
// absolute path and checksum is not implemented yet.
package opencode

import (
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

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
	if req.Kind == kindRetry {
		return retryAnswer(req), "", core.Decision{}
	}
	if field := invalidRouteField(req); field != "" {
		resp := failOpen()
		resp.InvalidField = field
		return resp, "", core.Decision{}
	}
	var res core.Resolver
	if len(r) > 0 {
		res = r[0]
	}
	child := req.Correlation.Child
	session := core.ResolveSessionForID(harnessID, child, nil, nil)
	// Available is only sent by the bridge; an absent field means "no
	// restriction" so a direct hook call (tests, other callers) keeps working.
	if req.Available != nil {
		session = session.RestrictToAvailable(req.Available)
	}
	currentModel := sessionModelID(req, session)
	res = core.ResolveSessionModels(harnessID, session, res)
	decision := core.Route(req.TaskText, harnessID, currentModel, res)
	// The requested id is what the payload carried; provenance telemetry
	// compares it against the catalog allowlist instead of trusting it.
	decision.Baseline.RequestedID = req.Model.ID

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

// sessionModelID picks the id the allowlist knows. OpenCode's discovery names
// a model "provider/id" (`opencode models` prints that), while the plugin
// reports the pair OpenCode itself holds ("opencode" + "beam-free"). When only
// the qualified form is listed, that is the id the allowlist, the quota marks
// and the plan use; a list already holding the bare id keeps it.
func sessionModelID(req Request, session core.SessionList) string {
	id := req.Model.ID
	if !session.Known || req.Model.ProviderID == "" || session.Contains(id) {
		return id
	}
	qualified := req.Model.ProviderID + "/" + id
	if session.Contains(qualified) {
		return qualified
	}
	return id
}

// routed is the answer that moves the child onto another model. A gateway
// model is addressed by its gateway as provider (OpenCode's provider/model
// form: "openrouter" + the slug); a native model keeps the harness provider.
//
// OpenCode reports a session model as "provider/id" (session discovery does:
// "opencode/beam-free"), while switchModel takes the provider and the id
// separately. The harness prefix is removed here; the plugin splits any other
// provider prefix it knows (an "opencode-go/…" allowlist entry), so a model
// whose own id holds a slash ("deepseek/deepseek-v4.1-flash-1017") survives
// when its prefix is not a provider OpenCode has.
func routed(m core.Model) Response {
	provider := m.Gateway
	if provider == "" {
		provider = m.Harness
	}
	if provider == "" {
		provider = harnessID
	}
	id := m.ID
	if rest, ok := strings.CutPrefix(id, provider+"/"); ok && rest != "" {
		id = rest
	}
	return Response{
		V:          1,
		Action:     "route",
		Model:      &ResponseModel{ProviderID: provider, ID: id},
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

// retryAnswer decides whether a failed child is retried once on the model it
// asked for. Only a child a route moved is retried, and only at its first
// retry opportunity: the routed model is the one change Downshift made, so
// falling back to the requested model is the undo. Any other failure is left
// to the harness (allow, no change), so a failed child never costs a second
// attempt Downshift did not cause.
//
// attempt is OpenCode's count for the retry under consideration: the initial
// request is 1, so the first retry opportunity is 2 (the harness documents
// this, and a live probe confirmed it). Anything later is the harness's own
// retry policy, not the undo. An absent attempt reads as 0 and does not retry.
func retryAnswer(req Request) Response {
	r := req.Retry
	if !validRetry(req) || !r.Routed || r.Attempt != firstRetryAttempt {
		return Response{V: 1, Action: "allow", ReasonCode: reasonNoRetry}
	}
	return Response{
		V:          1,
		Action:     "retry",
		Model:      &ResponseModel{ProviderID: r.Requested.ProviderID, ID: r.Requested.ID},
		ReasonCode: reasonRetried,
	}
}
