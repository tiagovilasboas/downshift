// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package opencode

import (
	"encoding/json"

	"github.com/tiagovilasboas/downshift-ai/internal/lazyre"
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
	maxVersionLen      = 64
	maxCorrelationLen  = 64
	maxAgentLen        = 64
	maxProviderIDLen   = 64
	maxModelIDLen      = 128
	maxVariantLen      = 128
	maxTaskTextBytes   = 64 << 10
	maxAvailableModels = 256
	maxTsUtcLen        = 64
	maxDirectoryLen    = 4096
)

// Call kinds and their events, the wire v1 vocabulary.
const (
	kindRoute    = "route"
	kindShell    = "shell"
	kindRetry    = "retry"
	eventSpawn   = "subagent_spawn_intent"
	eventShell   = "shell_command"
	eventRetry   = "child_retry"
	maxShellText = 16 << 10
	// firstRetryAttempt is OpenCode's attempt count at the first retry
	// opportunity (the initial request is 1); only it is the undo.
	firstRetryAttempt = 2
)

// childIDPattern is the correlation.child contract: lowercase hex, 16..64
// characters (the plugin sends an HMAC-derived id).
var childIDPattern = lazyre.New(`^[a-f0-9]{16,64}$`)

// Request is one call the plugin forwards (wire v1). The kind selects which
// fields matter: "route" for a subagent spawn, "shell" for a shell tool call,
// "retry" for a failed child. Unknown JSON fields are dropped by the strict
// decode below.
type Request struct {
	V           int             `json:"v"`
	Kind        string          `json:"kind"`
	Harness     RequestHarness  `json:"harness"`
	Event       string          `json:"event"`
	Correlation RequestRelation `json:"correlation"`
	Agent       string          `json:"agent"`
	Model       RequestModel    `json:"model"`
	TaskText    string          `json:"taskText"`
	// Command belongs to a "shell" call. Directory is the session's project
	// directory on every call (the shared runner reads it through ProjectDir).
	Command   string `json:"command"`
	Directory string `json:"directory"`
	// SessionDirectory belongs to a "shell" call: the session's project
	// directory, which can differ from the directory the shell runs in.
	SessionDirectory string `json:"sessionDirectory"`
	// Available belongs to a "route" call: the model pairs OpenCode reports as
	// selectable right now ("provider/id"). An empty or absent list means
	// "nothing is known", and Handle restricts the session to it.
	Available []string `json:"available"`
	// Retry belongs to a "retry" call.
	Retry *RequestRetry `json:"retry"`
	TsUtc string        `json:"tsUtc"`
}

// RequestRetry describes the failed child a retry call reports.
type RequestRetry struct {
	// Routed is true when a route answer moved this child onto another model.
	Routed bool `json:"routed"`
	// Requested is the model the child asked for before any route.
	Requested RequestModel `json:"requested"`
	// ErrorType and Status are the provider error the harness reported.
	ErrorType string `json:"errorType"`
	Status    int    `json:"status"`
	// Attempt counts the failed attempts of this child, starting at 1.
	Attempt int `json:"attempt"`
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
	// InvalidField names the request field that failed validation on a
	// fail-open answer. It never goes on the wire (json "-"); the CLI wire
	// records it in telemetry. The name only, never the value.
	InvalidField string `json:"-"`
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
	reasonRetried        = "routed_failed_retried"
	reasonNoRetry        = "no_retry"
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
		return Request{}, &fieldError{err: err, field: decodeField(err)}
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

// ProjectDir returns the session's project directory, so the shared runner
// loads that project's policy.
func (r Request) ProjectDir() string { return r.Directory }

// invalidRouteField names the first field that makes a request an invalid
// route request for this harness (a wrong value or one over its cap), or ""
// when the request is valid. The name only, never the value.
func invalidRouteField(req Request) string {
	checks := []struct {
		field string
		ok    bool
	}{
		{"v", req.V == 1},
		{"kind", req.Kind == kindRoute},
		{"harness.id", req.Harness.ID == harnessID},
		{"event", req.Event == eventSpawn},
		{"harness.version", len(req.Harness.Version) <= maxVersionLen},
		{"correlation.child", childIDPattern.MatchString(req.Correlation.Child)},
		{"correlation.parent", len(req.Correlation.Parent) <= maxCorrelationLen},
		{"agent", len(req.Agent) <= maxAgentLen},
		{"model.providerID", len(req.Model.ProviderID) <= maxProviderIDLen},
		{"model.id", req.Model.ID != "" && len(req.Model.ID) <= maxModelIDLen},
		{"model.variant", len(req.Model.Variant) <= maxVariantLen},
		{"taskText", req.TaskText != "" && len(req.TaskText) <= maxTaskTextBytes},
		{"tsUtc", len(req.TsUtc) <= maxTsUtcLen},
		{"available", validAvailable(req.Available)},
	}
	for _, c := range checks {
		if !c.ok {
			return c.field
		}
	}
	return ""
}

func validAvailable(available []string) bool {
	if len(available) > maxAvailableModels {
		return false
	}
	for _, id := range available {
		if len(id) == 0 || len(id) > maxModelIDLen*2 {
			return false
		}
	}
	return true
}

// validRetry reports whether the decoded request is a well-formed retry call
// for this harness: the same wire version, harness, event and child id shape
// as the other kinds, and a requested model within the field caps.
func validRetry(req Request) bool {
	r := req.Retry
	return req.V == 1 &&
		req.Kind == kindRetry &&
		req.Harness.ID == harnessID &&
		req.Event == eventRetry &&
		len(req.Harness.Version) <= maxVersionLen &&
		childIDPattern.MatchString(req.Correlation.Child) &&
		r != nil &&
		len(r.Requested.ProviderID) <= maxProviderIDLen &&
		r.Requested.ID != "" && len(r.Requested.ID) <= maxModelIDLen
}
