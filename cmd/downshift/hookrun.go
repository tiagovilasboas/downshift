// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
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
	"github.com/tiagovilasboas/downshift/internal/decisionintelligence"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/hookport"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

var hookReadTimeout = 2 * time.Second
var errHookReadTimeout = errors.New("hook input timeout")

// hookWire is how one harness receives the routed decision: the only
// per-harness step of the shared runner, chosen by the registry entry in
// harnesses.go. The runner itself holds no harness branch.
type hookWire interface {
	// failOpen writes the "allow unchanged" response. The runner calls it on
	// every failure, in the control group, when paused and under
	// DOWNSHIFT_NO_ROUTE, and never more than once per run.
	failOpen()
	// emit writes the routed response and returns the hook's exit code. A
	// non-zero code denies the spawn: every supported harness reads exit 2
	// as "block".
	emit(out any, correlationID string) (int, error)
}

// rewriteWire serves harnesses with an in-place rewrite channel: the adapter
// output is one JSON line on stdout and the exit code is always 0. The func
// value prints that harness's fail-open envelope.
type rewriteWire func()

func (w rewriteWire) failOpen() { w() }

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

func (policyWire) failOpen() {}

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
			wire.failOpen()
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
	ev, err := parse(data)
	if err != nil {
		return fail("INVALID_EVENT")
	}
	defer withHookDeadline()()
	if identified, ok := any(ev).(interface{ CorrelationIdentifier() string }); ok {
		correlationID = telemetry.CorrelationIDOrNew(identified.CorrelationIdentifier())
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
	// real spawn decisions, picked by a hash of the routing id, is classified
	// and logged but never rewritten. Same seam as no-route, same fail-open.
	fate.control = !fate.noRoute && decision.Harness != "" &&
		telemetry.InControlGroup(correlationID, telemetry.ControlShare())
	fate.noRoute = fate.noRoute || fate.control
	if fate.noRoute {
		wire.failOpen()
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
		fmt.Fprintf(os.Stderr, "downshift: honor=unobserved quota=%s usage=unobserved\n", telemetry.ProjectQuota(decision.QuotaStatus, decision.CreditHeld))
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
			if decision.BaselineFromSettings {
				// Inferred from settings: the adapter resolved it with the catalog.
				candidate = decision.RequestedID
			}
			if candidate != "" {
				// Route only populates CurrentModel when its resolver recognized
				// the original candidate. That makes this a catalog allowlist
				// check without coupling the shared hook runner to a catalog type.
				if decision.CurrentModel.ID != "" {
					event.FromModel = telemetry.ModelOrUnknown(candidate)
				} else {
					event.RequestedModelRaw = telemetry.RawModelOrInvalid(candidate)
				}
			}
		}
		event.SessionID = sessionHash
		if call, ok := any(ev).(interface{ ToolUseIdentifier() string }); ok {
			event.ToolUseHash = telemetry.HashSessionID(call.ToolUseIdentifier())
		}
		if typed, ok := any(ev).(interface{ AgentTypeIdentifier() string }); ok {
			event.AgentType = telemetry.AgentTypeLabel(typed.AgentTypeIdentifier())
		}
		if requested, ok := any(ev).(interface{ RequestedReasoningEffort() string }); ok {
			event.RequestedEffort = telemetry.EffortOrUnknown(requested.RequestedReasoningEffort())
		}
		telemetry.MarkUnchanged(&event)
		telemetry.Record(event)
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
	if d.Harness == "" || !d.SessionUnknown {
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
	fmt.Println(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`)
})

var printCursorAllow = rewriteWire(func() {
	fmt.Println(`{"permission":"allow"}`)
})

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
