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

	"github.com/tiagovilasboas/downshift/internal/adapters/opencode"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/decisionintelligence"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

var hookReadTimeout = 2 * time.Second
var errHookReadTimeout = errors.New("hook input timeout")

// hookGetenv is where the shared runner reads the DOWNSHIFT_* configuration
// that core routes by. It is read once per invocation (core.ConfigFromEnv)
// and pinned for the whole hook; tests swap it for a map.
var hookGetenv = os.Getenv

// hookWire is how one harness receives the routed decision: the only
// per-harness step of the shared runner, chosen by the registry entry in
// harnesses.go. The runner itself holds no harness branch.
type hookWire interface {
	// failOpen writes the "allow unchanged" response. cause is the failure
	// code on runner errors (recorded by wires that log fail-open) or
	// wireFailOpenNoRoute in the control group, when paused and under
	// DOWNSHIFT_NO_ROUTE. Never called more than once per run.
	failOpen(cause, correlationID string)
	// emit writes the routed response and returns the hook's exit code. A
	// non-zero code denies the spawn: every supported harness reads exit 2
	// as "block".
	emit(out any, correlationID string) (int, error)
}

// wireFailOpenNoRoute is the failOpen cause for "no route was attempted"
// (control group, per-session pause, DOWNSHIFT_NO_ROUTE): not a failure.
const wireFailOpenNoRoute = "no_route"

// rewriteWire serves harnesses with an in-place rewrite channel: the adapter
// output is one JSON line on stdout and the exit code is always 0. The func
// value prints that harness's fail-open envelope.
type rewriteWire func()

func (w rewriteWire) failOpen(_, _ string) { w() }

func (w rewriteWire) emit(out any, _ string) (int, error) {
	encoded, err := json.Marshal(out)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(os.Stdout, "%s\n", encoded)
	return 0, nil
}

// policyOutput is the adapter output of a policy-mode harness, one whose hook
// contract is exit-code only (no updated_input): allow unchanged or deny.
type policyOutput interface {
	// Denied reports whether the spawn is blocked and, if so, the message
	// relayed to the agent on stderr.
	Denied() (blocked bool, message string)
}

// policyWire serves policy-mode harnesses: exit 0 allows the spawn unchanged
// with nothing on stdout; exit 2 blocks it and the stderr line is what the
// harness relays to the LLM. Fail-open is silence, so a router error can
// never be read as a block.
type policyWire struct{}

func (policyWire) failOpen(_, _ string) {}

func (policyWire) emit(out any, correlationID string) (int, error) {
	o, ok := out.(policyOutput)
	if !ok {
		return 0, fmt.Errorf("policy wire: %T does not implement Denied", out)
	}
	if blocked, message := o.Denied(); blocked {
		fmt.Fprintf(os.Stderr, "downshift: correlation_id=%s %s\n", correlationID, message)
		return 2, nil
	}
	return 0, nil
}

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
	// after the panic recovery below. Fail-open, never waits (scorerefresh.go).
	defer maybeRefreshScores()
	correlationID := telemetry.NewCorrelationID()
	responded := false
	fail := func(code string) int {
		telemetry.Record(telemetry.Failure(correlationID, buildVersion, harness, code))
		if !responded {
			wire.failOpen(code, correlationID)
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
		if _, err := wire.emit(out, correlationID); err != nil {
			return fail("OUTPUT_ENCODE_ERROR")
		}
		responded = true
		return 0
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
	out, note, decision := handle(ev)
	fate := spawnFate{note: note}
	// Control-group mode for before/after comparison: classify as usual
	// but allow the spawn untouched, recording the decision as a
	// "baseline" event that stats excludes from routed rates.
	fate.noRoute = os.Getenv("DOWNSHIFT_NO_ROUTE") == "1"
	// Per-session pause (`downshift session --pause`) routes like the control
	// group: one stat, fail open, never a block. Harnesses whose payload has no
	// session id cannot be paused per session.
	sessionHash := ""
	if identified, ok := any(ev).(interface{ SessionIdentifier() string }); ok {
		sessionHash = telemetry.HashSessionID(identified.SessionIdentifier())
	}
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
	if decision.Harness != "" {
		// Spawn-time surface. Honor and usage stay unobserved until that
		// harness's own post-spawn port records them. One line, spawn events only.
		fmt.Fprintf(os.Stderr, "downshift: honor=unobserved quota=%s usage=unobserved\n", telemetry.ProjectQuota(decision.Session.QuotaStatus, decision.Session.CreditHeld))
		event := telemetry.FromDecision(decision, correlationID, buildVersion)
		fate.recordOutcome(&event)
		shadow := decisionintelligence.Evaluate(decisionintelligence.Signals{
			BaseTier:             decision.Tier,
			ClassifierConfidence: confidenceValue(decision.Confident),
		})
		event.DecisionIntelligence = &telemetry.ShadowRecommendation{
			Tier:              shadow.Tier.String(),
			Confidence:        shadow.Confidence,
			Reasons:           shadow.Reasons,
			RequiresReview:    shadow.RequiresReview,
			BudgetConstrained: shadow.BudgetConstrained,
			Apply:             shadow.Apply,
		}
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
		if len(taskText) > 0 {
			if id, err := training.RecordRoutedDecision(taskText[0](ev), decision); err == nil && id != "" {
				fmt.Fprintf(os.Stderr, "downshift: feedback id %s (run `downshift feedback %s success|retry|failed` after review)\n", id, id)
			}
		}
	}
	return rc
}

// runHonor is the post-spawn honor runner. The child already started, so any
// read, size, panic, or encode failure prints the neutral response ({}) and
// exits 0. A nil Honor func is unobserved: same stdout, plus one stderr line.
// Telemetry stays inside the port func (DOWNSHIFT_EVENT_LOG respected there).
func runHonor(in io.Reader, port hookport.Port, catalog core.Resolver) (rc int) {
	responded := false
	neutral := func() int {
		if !responded {
			fmt.Println(`{}`)
			responded = true
		}
		return 0
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error in honor, ignored")
			rc = neutral()
		}
	}()
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil || len(data) > maxHookPayloadBytes {
		return neutral()
	}
	if owner := foreignPayloadOwner(port.ID, data); owner != "" {
		fmt.Fprint(os.Stderr, foreignPayloadNote(port.ID, owner))
		return neutral()
	}
	obs := hookport.ObserveHonor(port, data, buildVersion, catalog)
	if !obs.Observed {
		fmt.Fprintf(os.Stderr, "downshift: honor unobserved for %s\n", port.ID)
		return neutral()
	}
	encoded, err := json.Marshal(obs.Stdout)
	if err != nil {
		return neutral()
	}
	responded = true
	fmt.Fprintf(os.Stdout, "%s\n", encoded)
	if obs.Note != "" {
		fmt.Fprintf(os.Stderr, "downshift: %s\n", obs.Note)
	}
	return 0
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

func confidenceValue(confident bool) float64 {
	if confident {
		return 1
	}
	return 0
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

// --- Harness-specific fail-open responses ---
// Each harness has a different envelope for "allow unchanged". These are the
// minimal valid JSON outputs that let the tool call proceed unmodified.

var printAllow = rewriteWire(func() {
	// No permissionDecision: an explicit allow would override the user's ask
	// rules. An empty object leaves the harness's own permission flow intact.
	fmt.Println(`{}`)
})

var printCursorAllow = rewriteWire(func() {
	fmt.Println(`{"permission":"allow"}`)
})

// openCodeWire serves the OpenCode route contract: one JSON object on stdout
// and the exit code always 0. Unlike the rewrite wires, fail-open is a valid
// allow answer the plugin caches: under no-route it answers reasonCode
// "no_route" (the decision was recorded as a baseline), and on any internal
// error it answers reasonCode "fail_open" and records the fail-open outcome
// so the rate is visible in stats. The adapter's own invalid-request answer
// ("fail_open" from Handle) is recorded here too, once.
type openCodeWire struct{}

func (openCodeWire) failOpen(cause, correlationID string) {
	if cause == wireFailOpenNoRoute {
		fmt.Printf("{\"v\":1,\"action\":\"allow\",\"reasonCode\":%q,\"ttlMs\":15000}\n", wireFailOpenNoRoute)
		return
	}
	telemetry.Record(telemetry.FailOpen(correlationID, buildVersion, "opencode", cause))
	fmt.Printf("{\"v\":1,\"action\":\"allow\",\"reasonCode\":%q}\n", opencode.ReasonFailOpen)
}

func (openCodeWire) emit(out any, correlationID string) (int, error) {
	resp, ok := out.(opencode.Response)
	if !ok {
		return 0, fmt.Errorf("opencode wire: %T is not a route response", out)
	}
	if resp.ReasonCode == opencode.ReasonFailOpen {
		telemetry.Record(telemetry.FailOpen(correlationID, buildVersion, "opencode", "INVALID_REQUEST"))
	}
	encoded, err := json.Marshal(resp)
	if err != nil {
		return 0, err
	}
	fmt.Fprintf(os.Stdout, "%s\n", encoded)
	return 0, nil
}

// runUsage is the post-spawn usage runner. The child already finished, so it
// never blocks: a read error, an oversized payload, or a panic exits 0 with
// no stdout. A port may return a stdout (the child-end line); otherwise none is
// printed. A nil Usage func is unobserved and prints one stderr line.
func runUsage(in io.Reader, port hookport.Port, catalog core.Resolver) (rc int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error in usage, ignored")
			rc = 0
		}
	}()
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil || len(data) > maxHookPayloadBytes {
		return 0
	}
	if owner := foreignPayloadOwner(port.ID, data); owner != "" {
		fmt.Fprint(os.Stderr, foreignPayloadNote(port.ID, owner))
		return 0
	}
	obs := hookport.ObserveUsage(port, data, buildVersion, catalog)
	if !obs.Observed {
		fmt.Fprintf(os.Stderr, "downshift: usage unobserved for %s\n", port.ID)
		return 0
	}
	if obs.Stdout != nil { // the child-end line, when the port has one to show
		if encoded, err := json.Marshal(obs.Stdout); err == nil {
			fmt.Fprintf(os.Stdout, "%s\n", encoded)
		}
	}
	if obs.Note != "" {
		fmt.Fprintf(os.Stderr, "downshift: %s\n", obs.Note)
	}
	return 0
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
