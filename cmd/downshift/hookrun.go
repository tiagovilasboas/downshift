// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/routing/agenttier"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

var hookReadTimeout = 2 * time.Second
var errHookReadTimeout = errors.New("hook input timeout")

// hookGetenv is where the shared runner reads the DOWNSHIFT_* configuration
// that core routes by. It is read once per invocation (core.ConfigFromEnv)
// and pinned for the whole hook; tests swap it for a map.
var hookGetenv = os.Getenv

// runHookAdapter is the single hook-runner template shared by all harness
// adapters, rewrite and policy mode alike. It reads a JSON event from in,
// calls handle, hands the result to wire, and records telemetry for any
// decision with a non-empty harness. Any failure prints the harness-specific
// fail-open response and exits 0 — the spawn must never be blocked by a
// router error.
func runHookAdapter[E any](
	harness string,
	in io.Reader,
	parse func([]byte) (E, error),
	handle func(E) (any, string, core.Decision),
	wire hookWire,
	taskText ...func(E) string,
) (rc int) {
	// Registered first so it runs last: after the response is written and
	// after the panic recovery below. Fail-open, never waits.
	// Same rule for every harness: a stale model list starts a detached refresh.
	defer maybeRefreshDiscovery(harness)
	correlationID := telemetry.NewCorrelationID()
	responded := false
	// After repeated identical failures the circuit opens (hook_circuit.go):
	// routing stops for this harness and no further failure is recorded.
	breaker := &hookCircuit{harness: harness}
	fail := func(code string, invalidField ...string) int {
		cause := code
		if breaker.hold(correlationID) {
			cause = wireFailOpenCircuit
		} else {
			ev := telemetry.Failure(correlationID, telemetryVersion(), harness, code)
			if len(invalidField) > 0 {
				ev = ev.WithInvalidField(invalidField[0])
			}
			telemetry.Record(ev)
		}
		if !responded {
			wire.failOpen(cause, correlationID)
			responded = true
		}
		return 0
	}
	// A Go panic exits with status 2, which every supported harness treats as
	// "block this tool call". Recover so a router bug can never block a
	// spawn: answer with the fail-open response (once) and exit 0.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error, spawn allowed unchanged")
			rc = fail("PANIC")
		}
	}()
	// Configuration is read once, here, at the edge: core and every shared
	// layer below it see one snapshot for the whole invocation, restored on
	// return like the hook deadline and the project context.
	defer core.SetConfig(core.ConfigFromEnv(hookGetenv))()
	// Harness hook payloads contain task text. Bound the read so a malformed or
	// hostile stdin cannot make the persistent harness process exhaust memory.
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil {
		if errors.Is(err, errHookReadTimeout) {
			return fail("INPUT_TIMEOUT")
		}
		return fail("READ_ERROR")
	}
	if len(data) > maxHookPayloadBytes {
		return fail("PAYLOAD_TOO_LARGE")
	}
	// Another harness's copy of a spawn its own hook already handles: allow it
	// unchanged and record nothing, so one spawn is one decision.
	if owner := foreignPayloadOwner(harness, data); owner != "" {
		wire.failOpen(wireFailOpenNoRoute, correlationID)
		responded = true
		fmt.Fprint(os.Stderr, foreignPayloadNote(harness, owner))
		return 0
	}
	ev, err := parse(data)
	if err != nil {
		return fail("INVALID_EVENT", parseErrorField(err))
	}
	// A shell-tool call is not a spawn: the opt-in exec wrapper may rewrite
	// its command (shellrewrite.go), and it is never routed or denied.
	if out, isShell := shellRewrite(harness, ev); isShell {
		if out == nil {
			wire.failOpen(wireFailOpenNoRoute, correlationID)
			responded = true
			return 0
		}
		code, err := wire.emit(out, correlationID)
		if err != nil {
			return fail("OUTPUT_ENCODE_ERROR")
		}
		responded = true
		return code
	}
	if identified, ok := any(ev).(interface{ CorrelationIdentifier() string }); ok {
		correlationID = telemetry.CorrelationIDOrNew(identified.CorrelationIdentifier())
	}
	toolUseID := ""
	if call, ok := any(ev).(interface{ ToolUseIdentifier() string }); ok {
		toolUseID = call.ToolUseIdentifier()
	}
	// One shared seam for project context: adapters only say where their
	// payload keeps the working directory (optional ProjectDir method).
	projectDir := ""
	if p, ok := any(ev).(interface{ ProjectDir() string }); ok {
		projectDir = p.ProjectDir()
	}
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	defer hookctx.SetProjectDir(projectDir)()
	sessionHash := ""
	if identified, ok := any(ev).(interface{ SessionIdentifier() string }); ok {
		sessionHash = telemetry.HashSessionID(identified.SessionIdentifier())
	}
	defer hookctx.SetSessionID(sessionHash)()
	if breaker.hold(correlationID) {
		wire.failOpen(wireFailOpenCircuit, correlationID)
		responded = true
		return 0
	}
	out, note, decision := handle(ev)
	fate := spawnFate{note: note}
	// The per-session pause (`downshift session --pause`) is the only way a
	// spawn goes unrouted, and only because the user asked for it: the decision
	// is classified and logged, the spawn fails open and is never blocked.
	// Harnesses whose payload has no session id cannot be paused per session.
	// No environment variable and no sampling skips routing.
	fate.paused = telemetry.SessionPaused(sessionHash)
	if fate.paused {
		wire.failOpen(wireFailOpenNoRoute, correlationID)
		responded = true
	} else {
		code, err := wire.emit(out, correlationID)
		if err != nil {
			return fail("OUTPUT_ENCODE_ERROR")
		}
		responded = true
		rc = code
		fate.denied = code != 0
	}
	fmt.Fprint(os.Stderr, fate.stderrLine(correlationID))
	warnSessionUnknown(decision)
	// Record telemetry only when a real routing decision was made.
	// A zero Decision (Harness == "") means the event was not a subagent spawn.
	// A batch adapter reports one decision per entry; each gets its own record.
	for _, routed := range recordedDecisions(decision, out) {
		decision := routed
		if decision.Harness == "" {
			continue
		}
		// Spawn-time surface. Honor and usage stay unobserved until that
		// harness's own post-spawn port records them. One line, spawn events only.
		fmt.Fprintf(os.Stderr, "downshift: honor=unobserved quota=%s usage=unobserved\n", telemetry.ProjectQuota(decision.Session.QuotaStatus, decision.Session.CreditHeld))
		event := fromcore.FromDecision(decision, correlationID, telemetryVersion())
		fate.recordOutcome(&event)
		// Provenance is stricter than routing: never replace an absent or
		// unrecognised requested model with the policy recommendation in the
		// evidence record. A catalog lookup is the local allowlist.
		event.FromModel = "unknown"
		if requested, ok := any(ev).(interface{ RequestedModel() string }); ok {
			candidate := requested.RequestedModel()
			if decision.Baseline.FromSettings || decision.Baseline.Pinned {
				// Inferred from settings, or fixed by an agent definition: the
				// adapter resolved it with the catalog.
				candidate = decision.Baseline.RequestedID
			}
			if candidate != "" {
				// Route only populates CurrentModel when its resolver recognized
				// the original candidate. That makes this a catalog allowlist
				// check without coupling the shared hook runner to a catalog type.
				if decision.Baseline.CurrentModel.ID != "" {
					event.FromModel = telemetry.ModelOrUnknown(candidate)
				} else {
					event.RequestedModelRaw = telemetry.RawModelOrInvalid(candidate)
				}
			}
		}
		event.SessionID = sessionHash
		event.ToolUseHash = telemetry.HashSessionID(toolUseID)
		event.Entry = telemetry.EntryHook
		if identified, ok := any(ev).(interface{ EntryIdentifier() string }); ok {
			event.Entry = telemetry.NormalizeEntry(identified.EntryIdentifier())
		}
		if typed, ok := any(ev).(interface{ AgentTypeIdentifier() string }); ok {
			event.AgentType = telemetry.AgentTypeLabel(typed.AgentTypeIdentifier())
			event.AgentName = agenttier.NameOf(typed.AgentTypeIdentifier(), squadPrefix(decision.Harness))
		}
		if typed, ok := any(ev).(interface{ PermissionModeIdentifier() string }); ok {
			event.PermissionMode = telemetry.PermissionModeLabel(typed.PermissionModeIdentifier())
		}
		if typed, ok := any(ev).(interface{ ClientIdentifier() string }); ok {
			event.Client = telemetry.ClientOrDrop(typed.ClientIdentifier())
		}
		if requested, ok := any(ev).(interface{ RequestedReasoningEffort() string }); ok {
			event.RequestedEffort = telemetry.EffortOrUnknown(requested.RequestedReasoningEffort())
		}
		stampParentModel(&event, ev)
		telemetry.MarkUnchanged(&event)
		telemetry.Record(event)
		// One visible line and one live record per spawn, for every harness:
		// what Downshift did (switched, held, observed). Fail-open, best effort.
		spawn := telemetry.NewSpawnRecord(event)
		if !telemetry.Quiet() {
			fmt.Fprintf(os.Stderr, "downshift: spawn %s\n", spawn.Line())
		}
		_ = telemetry.WriteLive(spawn)
	}
	return rc
}

// parseErrorField is the payload field a parse error names, when the adapter's
// error carries one (an optional InvalidField method); "" otherwise. The name
// only, never a value.
func parseErrorField(err error) string {
	var named interface{ InvalidField() string }
	if errors.As(err, &named) {
		return named.InvalidField()
	}
	return ""
}
