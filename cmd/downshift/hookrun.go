// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
	"github.com/tiagovilasboas/downshift/internal/telemetry/fromcore"
)

var hookReadTimeout = 2 * time.Second
var errHookReadTimeout = errors.New("hook input timeout")

// hookGetenv is where the shared runner reads the DOWNSHIFT_* configuration
// that core routes by. It is read once per invocation (core.ConfigFromEnv)
// and pinned for the whole hook; tests swap it for a map.
var hookGetenv = os.Getenv

// spawnFate is what the runner did with one spawn. The stderr line and the
// telemetry outcome both read from it, so they cannot disagree.
type spawnFate struct {
	// noRoute means the spawn was allowed unchanged by DOWNSHIFT_NO_ROUTE, a
	// session pause or the control group.
	noRoute bool
	paused  bool
	control bool
	// denied means the wire exited non-zero and the harness blocks the spawn.
	denied bool
	// note is the adapter's summary, set only when it rewrote or blocked.
	note string
}

// stderrLine is the one line that says what happened, or "" when the spawn
// ran untouched and the adapter had nothing to say.
func (f spawnFate) stderrLine(correlationID string) string {
	switch {
	case f.paused:
		return fmt.Sprintf("downshift: correlation_id=%s routing paused for this session, spawn unchanged (downshift session --resume)\n", correlationID)
	case f.control:
		return fmt.Sprintf("downshift: correlation_id=%s control group, spawn unchanged (DOWNSHIFT_CONTROL_SHARE=0 disables)\n", correlationID)
	case f.denied:
		// The wire already relayed the block message on stderr.
		return ""
	case f.note != "":
		return fmt.Sprintf("downshift: correlation_id=%s %s\n", correlationID, f.note)
	}
	return ""
}

// recordOutcome sets the event outcome and reason codes. A FromDecision event
// starts as a rewrite; only the fates below change that.
func (f spawnFate) recordOutcome(event *telemetry.Event) {
	switch {
	case f.noRoute:
		event.Outcome = telemetry.OutcomeBaseline
		if f.paused {
			event.ReasonCodes = append(event.ReasonCodes, telemetry.ReasonPaused)
			sort.Strings(event.ReasonCodes)
		}
		if f.control {
			event.ReasonCodes = append(event.ReasonCodes, telemetry.ReasonControl)
			sort.Strings(event.ReasonCodes)
		}
	case f.denied:
		event.Outcome = telemetry.OutcomeBlocked
	case f.note == "":
		// Adapters return a note only when they rewrote or blocked the spawn.
		event.Outcome = telemetry.OutcomeAllow
	}
}

// recordedDecisions returns the routing decisions one spawn produced: every
// entry when the adapter output reports them (a spawn_run batch), otherwise
// the single decision returned by the handler.
func recordedDecisions(first core.Decision, out any) []core.Decision {
	if multi, ok := out.(interface{ Decisions() []core.Decision }); ok && multi.Decisions() != nil {
		return multi.Decisions()
	}
	return []core.Decision{first}
}

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
	fail := func(code string) int {
		cause := code
		if breaker.hold(correlationID) {
			cause = wireFailOpenCircuit
		} else {
			telemetry.Record(telemetry.Failure(correlationID, buildVersion, harness, code))
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
		return fail("INVALID_EVENT")
	}
	defer withHookDeadline()()
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
	sentCorrelation := false
	if identified, ok := any(ev).(interface{ CorrelationIdentifier() string }); ok {
		sent := identified.CorrelationIdentifier()
		correlationID = telemetry.CorrelationIDOrNew(sent)
		sentCorrelation = correlationID == sent
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
	// Control-group mode for before/after comparison: classify as usual
	// but allow the spawn untouched, recording the decision as a
	// "baseline" event that stats excludes from routed rates.
	fate.noRoute = os.Getenv("DOWNSHIFT_NO_ROUTE") == "1"
	// Per-session pause (`downshift session --pause`) routes like the control
	// group: one stat, fail open, never a block. Harnesses whose payload has no
	// session id cannot be paused per session.
	fate.paused = !fate.noRoute && telemetry.SessionPaused(sessionHash)
	fate.noRoute = fate.noRoute || fate.paused
	// Built-in control group (`downshift report`): a deterministic share of
	// real spawn decisions, picked by a hash of the spawn's control key, is
	// classified and logged but never rewritten. Same seam as no-route, same fail-open.
	fate.control = !fate.noRoute && decision.Harness != "" &&
		telemetry.InControlGroup(controlKey(correlationID, sentCorrelation, toolUseID), telemetry.ControlShare())
	fate.noRoute = fate.noRoute || fate.control
	if fate.noRoute {
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
		event := fromcore.FromDecision(decision, correlationID, buildVersion)
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

// foreignPayloadOwner names the harness that sent a payload to the hook of
// harness when it is not that harness's own (core.ForeignPayloadOwner on the
// declared PayloadMarkers), or "". Only top-level keys are read; a payload that
// is not a JSON object is left to the normal parse path.
func foreignPayloadOwner(harness string, data []byte) string {
	var top map[string]json.RawMessage
	if json.Unmarshal(data, &top) != nil {
		return ""
	}
	return core.ForeignPayloadOwner(harness, func(key string) bool {
		_, ok := top[key]
		return ok
	})
}

func foreignPayloadNote(harness, owner string) string {
	return fmt.Sprintf("downshift: %s payload reached the %s hook; left to the %s hook, nothing recorded\n", owner, harness, owner)
}

// withHookDeadline installs the global classification deadline that bounds
// every external helper command, and returns its cleanup.
func withHookDeadline() func() {
	ctx, cancel := context.WithTimeout(context.Background(), hookctx.HookBudget)
	restore := hookctx.Set(ctx)
	return func() {
		restore()
		cancel()
	}
}

// warnSessionUnknown prints one stderr line per hook call when the adapter
// could not resolve a session allowlist: without it the hook never rewrites,
// which otherwise looks like a silent no-op install.
func warnSessionUnknown(d core.Decision) {
	if d.Harness == "" || !d.Session.Unknown {
		return
	}
	fmt.Fprintf(os.Stderr, "downshift: no session allowlist for %s, so no model is rewritten. "+
		"Add \"%s\": [<model ids>] to %s (see docs/reference/session-models.md)\n",
		d.Harness, d.Harness, sessionModelsPathForDisplay())
}

func sessionModelsPathForDisplay() string {
	return paths.DisplaySessionModels()
}

// readHookPayload prevents an unresponsive hook stdin from blocking a spawn.
// The reader goroutine may remain blocked until process exit, but the hook
// returns its fail-open response at the deadline and the short-lived process
// is then reclaimed by the harness OS process lifecycle.
func readHookPayload(in io.Reader, limit int64) ([]byte, error) {
	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(in, limit))
		done <- result{data: data, err: err}
	}()
	select {
	case result := <-done:
		return result.data, result.err
	case <-time.After(hookReadTimeout):
		return nil, errHookReadTimeout
	}
}

// controlKey is the id the control group hashes for one spawn: the harness's
// own correlation id when it sent a valid one, else its tool-use id, else the
// per-call correlation id. Claude Code, Cursor and Codex send only a tool-use
// id, and the settings hook and the mod entry see the same one, so both copies
// of a spawn and any retry land on the same side. The key is only hashed in
// memory; the tool-use id goes through the validated session hash first.
func controlKey(correlationID string, sentByHarness bool, toolUseID string) string {
	if sentByHarness {
		return correlationID
	}
	if hash := telemetry.HashSessionID(toolUseID); hash != "" {
		return "tool-use:" + hash
	}
	return correlationID
}
