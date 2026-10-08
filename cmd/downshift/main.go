// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Command downshift is the harness-downshift binary. It runs as a hook: a
// harness pipes a JSON event on stdin, downshift classifies the subagent task
// and prints the steering JSON on stdout that rewrites the subagent's model.
//
// Usage as a Claude Code PreToolUse hook:
//
//	{ "type": "command", "command": "downshift claude-code" }
//
// It also has a `try` subcommand to test classification from the terminal:
//
//	downshift try "rename the variable userId"
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/adapters/antigravity"
	"github.com/tiagovilasboas/downshift/internal/adapters/claudecode"
	"github.com/tiagovilasboas/downshift/internal/adapters/codex"
	"github.com/tiagovilasboas/downshift/internal/adapters/cursor"
	"github.com/tiagovilasboas/downshift/internal/adapters/kirocrew"
	"github.com/tiagovilasboas/downshift/internal/benchmark"
	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/decisionintelligence"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/models"
	"github.com/tiagovilasboas/downshift/internal/paths"
	"github.com/tiagovilasboas/downshift/internal/routingv2/classifier"
	"github.com/tiagovilasboas/downshift/internal/routingv2/training"
	dsserver "github.com/tiagovilasboas/downshift/internal/server"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// buildVersion, buildCommit and buildDate are set by release builds with
// -ldflags (see .goreleaser.yml). buildVersion is included in prompt-free
// routing telemetry to make policy incidents reproducible.
var (
	buildVersion = "dev"
	buildCommit  = ""
	buildDate    = ""
)

var hookReadTimeout = 2 * time.Second
var errHookReadTimeout = errors.New("hook input timeout")

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage(os.Stderr)
		os.Exit(2)
	}

	// Load the effective catalog once at startup — embedded JSON with optional
	// user override at ~/.harness-downshift/catalog.json. Fail-open: if the
	// user file is invalid, the embedded catalog is used automatically.
	catalog := catalog.Load()

	switch args[0] {
	case "antigravity":
		os.Exit(runHookAdapter(
			os.Stdin,
			func(b []byte) (antigravity.Event, error) { var e antigravity.Event; return e, json.Unmarshal(b, &e) },
			func(e antigravity.Event) (any, string, core.Decision) { return antigravity.Handle(e, catalog) },
			func() { fmt.Println(`{"decision":"allow"}`) },
			func(e antigravity.Event) string { return e.TaskText() },
		))
	case "claude-code":
		os.Exit(runHookAdapter(
			os.Stdin,
			func(b []byte) (claudecode.Event, error) { var e claudecode.Event; return e, json.Unmarshal(b, &e) },
			func(e claudecode.Event) (any, string, core.Decision) { return claudecode.Handle(e, catalog) },
			printAllow,
			func(e claudecode.Event) string { return e.TaskText() },
		))
	case "claude-code-post-tool-use":
		os.Exit(runClaudePostToolUse(os.Stdin, catalog))
	case "claude-code-subagent-stop":
		os.Exit(runClaudeSubagentStop(os.Stdin, catalog))
	case "cursor":
		os.Exit(runHookAdapter(
			os.Stdin,
			func(b []byte) (cursor.Event, error) { var e cursor.Event; return e, json.Unmarshal(b, &e) },
			func(e cursor.Event) (any, string, core.Decision) { return cursor.Handle(e, catalog) },
			printCursorAllow,
			func(e cursor.Event) string { return e.TaskText() },
		))
	case "codex":
		os.Exit(runHookAdapter(
			os.Stdin,
			func(b []byte) (codex.Event, error) { var e codex.Event; return e, json.Unmarshal(b, &e) },
			func(e codex.Event) (any, string, core.Decision) { return codex.Handle(e, catalog) },
			printCodexAllow,
			func(e codex.Event) string { return e.TaskText() },
		))
	case "kirocrew":
		os.Exit(runKiroCrewHook(os.Stdin, catalog))
	case "serve":
		// Find web/ next to the binary, then fall back to CWD/web
		exe, _ := os.Executable()
		webDir := filepath.Join(filepath.Dir(exe), "web")
		if _, err := os.Stat(webDir); err != nil {
			webDir = "web"
		}
		if err := dsserver.Run(dsserver.DefaultPort, webDir); err != nil {
			fmt.Fprintf(os.Stderr, "serve: %v\n", err)
			os.Exit(1)
		}
	case "try":
		os.Exit(runTry(catalog, args[1:]))
	case "models":
		os.Exit(runModels(catalog, args[1:]))
	case "stats":
		os.Exit(runStats(args[1:]))
	case "benchmark":
		os.Exit(runBenchmark(args[1:]))
	case "eval-outcome":
		os.Exit(runEvalOutcome(args[1:], os.Stdout, os.Stderr))
	case "shadow-report":
		os.Exit(runShadowReport(args[1:]))
	case "verification-report":
		os.Exit(runVerificationReport(args[1:], os.Stdout, os.Stderr))
	case "train":
		os.Exit(runTrain(args[1:]))
	case "feedback":
		os.Exit(runFeedback(args[1:]))
	case "doctor":
		os.Exit(runDoctor(os.Stdout, catalog))
	case "version", "--version":
		os.Exit(runVersion(os.Stdout))
	case "-h", "--help", "help":
		usage(os.Stdout)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", args[0])
		usage(os.Stderr)
		os.Exit(2)
	}
}

// runVersion prints the binary version, plus commit and build date when the
// release build stamped them.
func runVersion(w io.Writer) int {
	line := "downshift " + buildVersion
	if buildCommit != "" || buildDate != "" {
		line += fmt.Sprintf(" (commit %s, built %s)", buildCommit, buildDate)
	}
	fmt.Fprintln(w, line)
	return 0
}

// runHookAdapter is the single hook-runner template shared by all harness
// adapters. It reads a JSON event from in, calls handle, encodes the result
// to stdout, and records telemetry for any decision with a non-empty harness.
// Any failure prints the harness-specific fail-open response and exits 0 —
// the spawn must never be blocked by a router error.
func runHookAdapter[E any](
	in io.Reader,
	parse func([]byte) (E, error),
	handle func(E) (any, string, core.Decision),
	failOpen func(),
	taskText ...func(E) string,
) (rc int) {
	correlationID := telemetry.NewCorrelationID()
	responded := false
	fail := func(code string) int {
		telemetry.Record(telemetry.Failure(correlationID, buildVersion, code))
		if !responded {
			failOpen()
			responded = true
		}
		return 0
	}
	// A Go panic exits with status 2, which Claude Code and Codex treat as
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
	out, note, decision := handle(ev)
	// Control-group mode for before/after comparison: classify as usual
	// but allow the spawn untouched, recording the decision as a
	// "baseline" event that stats excludes from routed rates.
	noRoute := os.Getenv("DOWNSHIFT_NO_ROUTE") == "1"
	if noRoute {
		failOpen()
		responded = true
	} else {
		encoded, err := json.Marshal(out)
		if err != nil {
			return fail("OUTPUT_ENCODE_ERROR")
		}
		responded = true
		fmt.Fprintf(os.Stdout, "%s\n", encoded)
	}
	if note != "" {
		fmt.Fprintf(os.Stderr, "downshift: correlation_id=%s %s\n", correlationID, note)
	}
	warnSessionUnknown(decision)
	// Record telemetry only when a real routing decision was made.
	// A zero Decision (Harness == "") means the event was not a subagent spawn.
	if decision.Harness != "" {
		event := telemetry.FromDecision(decision, correlationID, buildVersion)
		switch {
		case noRoute:
			event.Outcome = telemetry.OutcomeBaseline
		case note == "":
			// Adapters return a note only when they rewrote the spawn.
			event.Outcome = telemetry.OutcomeAllow
		}
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
			if candidate != "" {
				// Route only populates CurrentModel when its resolver recognized
				// the original candidate. That makes this a catalog allowlist
				// check without coupling the shared hook runner to a catalog type.
				if decision.CurrentModel.ID != "" {
					event.FromModel = telemetry.ModelOrUnknown(candidate)
				}
			}
		}
		if identified, ok := any(ev).(interface{ SessionIdentifier() string }); ok {
			event.SessionID = telemetry.HashSessionID(identified.SessionIdentifier())
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
	return 0
}

// runClaudePostToolUse is the PostToolUse hook runner for Claude Code. It
// mirrors runHookAdapter's fail-open contract: the spawn already completed,
// so any read/parse/encode failure prints the neutral PostToolUse response
// ({}) and exits 0. Telemetry goes to the hermetic event log
// (DOWNSHIFT_EVENT_LOG respected inside the telemetry package).
func runClaudePostToolUse(in io.Reader, catalog core.Resolver) (rc int) {
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
			fmt.Fprintln(os.Stderr, "downshift: internal error in PostToolUse, ignored")
			rc = neutral()
		}
	}()
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil || len(data) > maxHookPayloadBytes {
		return neutral()
	}
	out, note, _ := claudecode.HandlePostToolUse(data, buildVersion, catalog)
	encoded, err := json.Marshal(out)
	if err != nil {
		return neutral()
	}
	responded = true
	fmt.Fprintf(os.Stdout, "%s\n", encoded)
	if note != "" {
		fmt.Fprintf(os.Stderr, "downshift: %s\n", note)
	}
	return 0
}

func confidenceValue(confident bool) float64 {
	if confident {
		return 1
	}
	return 0
}

// runKiroCrewHook is the exit-code runner for KiroCrew. Unlike runHookAdapter,
// KiroCrew's preToolUse contract has no updated_input channel: the only levers
// are exit 0 (allow) and exit 2 (block + stderr relayed to the LLM). So this
// runner never prints a rewrite to stdout — it classifies the spawn and, on a
// confident tier mismatch, returns exit 2 with an actionable respawn message.
// Fail-open is absolute: any read/parse error, or an allow decision, exits 0.
func runKiroCrewHook(in io.Reader, catalog core.Resolver) (rc int) {
	correlationID := telemetry.NewCorrelationID()
	fail := func(code string) int {
		telemetry.Record(telemetry.Failure(correlationID, buildVersion, code))
		return 0 // fail-open: never block a spawn on a router error
	}
	// A Go panic exits with status 2, which KiroCrew treats as "block".
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error, spawn allowed unchanged")
			rc = fail("PANIC")
		}
	}()

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

	var ev kirocrew.Event
	if err := json.Unmarshal(data, &ev); err != nil {
		return fail("INVALID_EVENT")
	}
	defer withHookDeadline()()
	correlationID = telemetry.CorrelationIDOrNew(ev.CorrelationIdentifier())

	out, note, decision := kirocrew.Handle(ev, catalog)
	warnSessionUnknown(decision)

	// Telemetry only when a real routing decision was made (Harness != "").
	// DOWNSHIFT_NO_ROUTE=1 records a control-group baseline and never blocks.
	if decision.Harness != "" {
		event := telemetry.FromDecision(decision, correlationID, buildVersion)
		// Policy mode never rewrites: a spawn is either blocked (exit 2) or
		// allowed unchanged (exit 0).
		switch {
		case os.Getenv("DOWNSHIFT_NO_ROUTE") == "1":
			event.Outcome = telemetry.OutcomeBaseline
		case out.Block:
			event.Outcome = telemetry.OutcomeBlocked
		default:
			event.Outcome = telemetry.OutcomeAllow
		}
		if currentID := decision.CurrentModel.ID; currentID != "" {
			event.FromModel = telemetry.ModelOrUnknown(currentID)
		} else {
			event.FromModel = "unknown"
		}
		telemetry.MarkUnchanged(&event)
		telemetry.Record(event)
		if id, err := training.RecordRoutedDecision(ev.TaskText(), decision); err == nil && id != "" {
			fmt.Fprintf(os.Stderr, "downshift: feedback id %s (run `downshift feedback %s success|retry|failed` after review)\n", id, id)
		}
	}

	if out.Block && os.Getenv("DOWNSHIFT_NO_ROUTE") != "1" {
		// stderr is what KiroCrew relays to the LLM on exit 2.
		fmt.Fprintf(os.Stderr, "downshift: correlation_id=%s %s\n", correlationID, out.Message)
		return 2
	}
	if note != "" {
		fmt.Fprintf(os.Stderr, "downshift: correlation_id=%s %s\n", correlationID, note)
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
		"Add \"%s\": [<model ids>] to %s (see docs/session-models.md)\n",
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

type catalogEntryReader interface {
	Entries() []catalog.Entry
}

// runTry classifies a prompt from the command line for quick testing.
func runTry(resolver core.Resolver, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift try \"<task prompt>\" [harness] [current-model]")
		return 2
	}
	prompt := args[0]
	if strings.TrimSpace(prompt) == "" {
		fmt.Fprintln(os.Stderr, "usage: downshift try \"<task prompt>\" [harness] [current-model]")
		return 2
	}
	harness := "claude-code"
	current := ""
	if len(args) >= 2 {
		harness = args[1]
	}
	if len(args) >= 3 {
		current = args[2]
	}
	if !knownTryHarness(resolver, harness) {
		fmt.Fprintf(os.Stderr, "unknown harness %q; valid harnesses: %s\n", harness, strings.Join(validTryHarnesses(resolver), ", "))
		return 2
	}

	// Grok uses a single model with configurable reasoning — report effort
	// instead of a model ID, and point at the config-based mechanism.
	if harness == "grok" {
		cls := core.Classify(prompt)
		tier := cls.Complexity.Tier()
		effort := core.EffortFor(tier)
		fmt.Printf("Task:       %s\n", prompt)
		fmt.Printf("Complexity: %s\n", cls.Complexity)
		fmt.Printf("Needs tier: %s\n", tier)
		fmt.Printf("Reasoning:  %s  (grok-4.6, configurable reasoning)\n", effort)
		fmt.Printf("→ set reasoning_effort=%q on the subagent role/persona in config.toml\n", effort.String())
		return 0
	}

	res, known := tryHook(harness, prompt, current, resolver)
	d := res.decision
	if !known {
		d = core.Route(prompt, harness, current, resolver)
	}
	fmt.Printf("Task:       %s\n", prompt)
	fmt.Printf("Complexity: %s\n", d.Complexity)
	fmt.Printf("Intent:     %s\n", d.Intent)
	fmt.Printf("Needs tier: %s\n", d.Tier)
	fmt.Printf("Recommend:  %s\n", d.Model.ID)
	if d.CurrentModel.ID != "" {
		fmt.Printf("Current:    %s\n", d.CurrentModel.ID)
	}
	fmt.Printf("Verdict:    %s\n", d.Verdict)
	fmt.Printf("Confident:  %t\n", d.Confident)
	if len(d.Corrections) > 0 {
		fmt.Printf("Held by:    %s\n", strings.Join(d.Corrections, ", "))
	}
	switch {
	case !known:
		fmt.Printf("Rewrite:    unknown harness %q (no hook adapter)\n", harness)
	case res.blocked:
		fmt.Printf("Hook:       block (exit 2) → respawn with %s\n", res.written)
	case res.written != "":
		fmt.Printf("Rewrite:    yes → %s\n", res.written)
	case d.SessionUnknown:
		fmt.Printf("Rewrite:    no: session unknown. Add \"%s\": [<model ids>] to %s (see docs/session-models.md)\n",
			harness, sessionModelsPathForDisplay())
	case d.Intent == core.PreservedIntent:
		fmt.Printf("Rewrite:    no (current model is explicit_only)\n")
	case len(d.Corrections) > 0:
		fmt.Printf("Rewrite:    no (held by guardrail)\n")
	default:
		fmt.Printf("Rewrite:    no (keep current model)\n")
	}
	if res.effort != "" {
		fmt.Printf("Effort:     → %s\n", res.effort)
	}
	fmt.Printf("→ %s\n", d.Summary())
	return 0
}

func knownTryHarness(resolver core.Resolver, harness string) bool {
	for _, name := range validTryHarnesses(resolver) {
		if harness == name {
			return true
		}
	}
	return false
}

func validTryHarnesses(resolver core.Resolver) []string {
	names := map[string]struct{}{"grok": {}}
	if reader, ok := resolver.(catalogEntryReader); ok {
		for _, entry := range reader.Entries() {
			if entry.Harness != "" {
				names[entry.Harness] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// tryResult is what the real hook adapter would emit for a prompt.
type tryResult struct {
	decision core.Decision
	written  string // model id the hook would write ("" for none)
	effort   string // reasoning effort the hook would write ("" for none)
	blocked  bool   // KiroCrew policy mode: exit 2
}

// tryHook runs the harness's real adapter on a synthetic payload so `try`
// reports exactly what the hook would do, including the session allowlist
// and every guardrail. known is false for a harness without an adapter.
func tryHook(harness, prompt, current string, catalog core.Resolver) (tryResult, bool) {
	input := func(promptKey string) json.RawMessage {
		m := map[string]any{promptKey: prompt}
		if current != "" {
			m["model"] = current
		}
		raw, _ := json.Marshal(m)
		return raw
	}
	written := func(raw json.RawMessage, key string) string {
		if len(raw) == 0 {
			return ""
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			return ""
		}
		v, _ := m[key].(string)
		return v
	}
	switch harness {
	case "claude-code":
		out, _, d := claudecode.Handle(claudecode.Event{ToolName: "Task", ToolInput: input("prompt")}, catalog)
		r := tryResult{decision: d}
		if out.HookSpecificOutput != nil {
			if w := written(out.HookSpecificOutput.UpdatedInput, "model"); w != current {
				r.written = w
			}
		}
		return r, true
	case "cursor":
		out, _, d := cursor.Handle(cursor.Event{ToolName: "Task", ToolInput: input("task")}, catalog)
		r := tryResult{decision: d}
		if w := written(out.UpdatedInput, "model"); w != current {
			r.written = w
		}
		return r, true
	case "codex":
		out, _, d := codex.Handle(codex.Event{ToolName: "spawn_agent", ToolInput: input("message")}, catalog)
		r := tryResult{decision: d}
		if out.HookSpecificOutput != nil {
			if w := written(out.HookSpecificOutput.UpdatedInput, "model"); w != current {
				r.written = w
			}
			r.effort = written(out.HookSpecificOutput.UpdatedInput, "reasoning_effort")
		}
		return r, true
	case "kirocrew":
		out, _, d := kirocrew.Handle(kirocrew.Event{ToolName: "spawn_run", ToolInput: input("task")}, catalog)
		r := tryResult{decision: d, blocked: out.Block}
		if out.Block {
			r.written = d.Model.ID
		}
		return r, true
	case "antigravity":
		sub := map[string]any{"Prompt": prompt}
		if current != "" {
			sub["Model"] = current
		}
		args, _ := json.Marshal(map[string]any{"Subagents": []any{sub}})
		out, _, d := antigravity.Handle(antigravity.Event{ToolCall: antigravity.ToolCall{Name: "invoke_subagent", Args: args}}, catalog)
		r := tryResult{decision: d}
		if len(out.Overwrite) > 0 {
			var m struct {
				Subagents []map[string]any `json:"Subagents"`
			}
			if json.Unmarshal(out.Overwrite, &m) == nil && len(m.Subagents) > 0 {
				if w, _ := m.Subagents[0]["Model"].(string); w != current {
					r.written = w
				}
			}
		}
		return r, true
	default:
		return tryResult{}, false
	}
}

// runModels dispatches the 'models' subcommands: list, check, pull.
func runModels(catalog models.CatalogReader, args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift models <list|check|pull>")
		return 2
	}
	switch args[0] {
	case "list":
		models.List(catalog, os.Stdout)
		return 0
	case "check":
		return models.Check(catalog, os.Stdout, os.Stderr)
	case "pull":
		return models.Pull(catalog, os.Stdout, os.Stderr)
	default:
		fmt.Fprintf(os.Stderr, "unknown models subcommand %q\n", args[0])
		return 2
	}
}

// runStats reads the local event log and prints a savings summary.
// Usage: downshift stats [--days=N] [--cost-per-unit=USD] [--export]
//
// --days=N           time window in days (default 30; 0 = all time)
// --cost-per-unit=X  convert normalised units to dollars at rate X per unit
// --export           print a pasteable JSON summary for multi-user evidence
func runStats(args []string) int {
	opts := telemetry.StatsOptions{Days: 30}
	export := false
	for _, arg := range args {
		switch {
		case arg == "--export":
			export = true
		case len(arg) > 7 && arg[:7] == "--days=":
			n, err := strconv.ParseFloat(arg[7:], 64)
			if err != nil || n < 0 {
				fmt.Fprintf(os.Stderr, "invalid --days value: %s\n", arg[7:])
				return 2
			}
			opts.Days = int(n)
		case len(arg) > 16 && arg[:16] == "--cost-per-unit=":
			v, err := strconv.ParseFloat(arg[16:], 64)
			if err != nil || v < 0 {
				fmt.Fprintf(os.Stderr, "invalid --cost-per-unit value: %s\n", arg[16:])
				return 2
			}
			opts.CostPerUnit = v
		default:
			fmt.Fprintf(os.Stderr, "unknown flag: %s\n", arg)
			return 2
		}
	}

	events, err := telemetry.ReadEvents()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading event log: %v\n", err)
		return 1
	}
	if export {
		out, err := telemetry.ExportJSON(telemetry.ExportSummary(events, opts.Days, time.Now().UTC()))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error exporting summary: %v\n", err)
			return 1
		}
		fmt.Println(out)
		return 0
	}
	if len(events) == 0 {
		fmt.Fprintln(os.Stderr, "no events recorded yet — run some subagent tasks first.")
		return 0
	}
	telemetry.PrintStats(events, opts, os.Stdout)
	return 0
}

// runBenchmark runs the classifier against a labelled task dataset and prints
// a confusion matrix + false-downshift rate.
// Usage: downshift benchmark <dataset.json> [--compare] [--report] [--min-tier-accuracy=0.60]
func runBenchmark(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift benchmark <dataset.json> [--compare] [--report] [--min-tier-accuracy=0.60]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "The dataset is a JSON array of {\"prompt\":\"...\",\"label\":\"TRIVIAL|SIMPLE|MEDIUM|COMPLEX\"} objects")
		fmt.Fprintln(os.Stderr, "(tier labels SMALL|MID|FRONTIER are accepted too).")
		fmt.Fprintln(os.Stderr, "Maintainer benchmark files live in downshift-labs; public summary: benchmark/REPORT.md.")
		fmt.Fprintln(os.Stderr, "Use --compare --candidate-weights=<file> to evaluate a candidate without activating it.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  --compare                Run both Legacy and CapabilityRouter v2 side by side")
		fmt.Fprintln(os.Stderr, "  --report                 Emit machine-readable JSON metrics summary to stdout")
		fmt.Fprintln(os.Stderr, "  --min-tier-accuracy=0.60 Exit with error (1) if tier routing accuracy is below threshold")
		fmt.Fprintln(os.Stderr, "  --gate                   Apply all gates (tier accuracy, FRONTIER→SMALL = 0, FRONTIER→MID max)")
		fmt.Fprintln(os.Stderr, "  --max-frontier-to-mid=0  With --gate: maximum FRONTIER→MID rate (default 0.70)")
		return 2
	}

	compare := false
	reportJSON := false
	gate := false
	minTierAccuracy := 0.0
	maxFrontierToMid := -1.0
	path := ""
	candidateWeights := ""
	for _, arg := range args {
		switch {
		case arg == "--compare":
			compare = true
		case arg == "--report":
			reportJSON = true
		case arg == "--gate":
			gate = true
		case strings.HasPrefix(arg, "--min-tier-accuracy="):
			valStr := strings.TrimPrefix(arg, "--min-tier-accuracy=")
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil || val < 0 || val > 1 {
				// A typo must not silently disable the gate.
				fmt.Fprintf(os.Stderr, "error: --min-tier-accuracy=%q must be a number between 0 and 1\n", valStr)
				return 2
			}
			minTierAccuracy = val
		case strings.HasPrefix(arg, "--max-frontier-to-mid="):
			valStr := strings.TrimPrefix(arg, "--max-frontier-to-mid=")
			val, err := strconv.ParseFloat(valStr, 64)
			if err != nil || val < 0 || val > 1 {
				fmt.Fprintf(os.Stderr, "error: --max-frontier-to-mid=%q must be a number between 0 and 1\n", valStr)
				return 2
			}
			maxFrontierToMid = val
		case strings.HasPrefix(arg, "--candidate-weights="):
			candidateWeights = strings.TrimPrefix(arg, "--candidate-weights=")
		default:
			path = arg
		}
	}

	if path == "" {
		fmt.Fprintln(os.Stderr, "error: dataset path required")
		return 2
	}

	if compare {
		return runBenchmarkCompare(path, candidateWeights)
	}

	tasks, err := benchmark.LoadDataset(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading dataset: %v\n", err)
		return 1
	}

	results := benchmark.Run(tasks, os.Stderr)
	if len(results) == 0 {
		// Every row was skipped (unknown labels) or the file is empty: a
		// report of zeros must not pass as a successful run.
		fmt.Fprintf(os.Stderr, "error: %s has no tasks with a recognised label (TRIVIAL|SIMPLE|MEDIUM|COMPLEX|SMALL|MID|FRONTIER)\n", path)
		return 1
	}
	report := benchmark.GenerateReport(results)

	if reportJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "error encoding report: %v\n", err)
			return 1
		}
	} else if !gate {
		matrix := benchmark.Build(results)
		benchmark.Print(results, matrix, os.Stdout)
	}

	gates := benchmark.DefaultGateThresholds()
	if minTierAccuracy > 0 {
		gates.MinTierAccuracy = minTierAccuracy
	}
	if maxFrontierToMid >= 0 {
		gates.MaxFrontierToMIDRate = maxFrontierToMid
	}
	if gate {
		ok, reasons := benchmark.EvaluateGates(report, gates)
		if !ok {
			for _, r := range reasons {
				fmt.Fprintf(os.Stderr, "benchmark gate failed: %s\n", r)
			}
			return 1
		}
	} else if minTierAccuracy > 0 && report.TierAccuracy < minTierAccuracy {
		fmt.Fprintf(os.Stderr, "benchmark gate failed: tier accuracy %.1f%% below minimum required %.1f%%\n",
			report.TierAccuracy*100, minTierAccuracy*100)
		return 1
	}

	return 0
}

// runBenchmarkCompare runs Legacy vs CapabilityRouter v2 side by side.
// Uses the v2 training dataset format (SMALL/MID/FRONTIER labels).
func runBenchmarkCompare(path, candidateWeights string) int {
	// Load dataset in v2 format (SMALL/MID/FRONTIER)
	v2ds, err := training.LoadDataset(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading v2 dataset: %v\n", err)
		return 1
	}
	if v2ds.Size() == 0 {
		fmt.Fprintln(os.Stderr, "error: comparison dataset must contain at least one task")
		return 2
	}

	legacyTasks, err := legacyTasksForComparison(v2ds.Examples)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return 2
	}

	// Legacy: map equivalent tiers to the old classifier's label vocabulary.
	legacyResults := benchmark.Run(legacyTasks, os.Stderr)

	// v2: run the capability router classifier
	clf := classifier.NewSoftmaxClassifierDefault()
	if candidateWeights != "" {
		weights, err := classifier.LoadWeightsFromFile(candidateWeights)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading candidate weights: %v\n", err)
			return 1
		}
		clf = classifier.NewSoftmaxClassifier(weights)
	}
	v2Actual := make([]core.Tier, v2ds.Size())
	v2Predicted := make([]core.Tier, v2ds.Size())

	for i, ex := range v2ds.Examples {
		v2Actual[i] = ex.Tier()
		probs, _ := clf.Classify(ex.Features)
		v2Predicted[i] = probs.MaxTier()
	}

	v2Metrics := training.Compute(v2Actual, v2Predicted)
	classCounts := map[core.Tier]int{}
	for _, tier := range v2Actual {
		classCounts[tier]++
	}

	// Compute legacy metrics in v2 terms (map COMPLEX→FRONTIER, etc.)
	legacyActual := make([]core.Tier, len(legacyResults))
	legacyPredicted := make([]core.Tier, len(legacyResults))
	for i, r := range legacyResults {
		// Convert legacy complexity to tier
		switch r.Task.Label {
		case "TRIVIAL":
			legacyActual[i] = core.TierSmall
		case "SIMPLE", "MEDIUM":
			legacyActual[i] = core.TierMid
		case "COMPLEX":
			legacyActual[i] = core.TierFrontier
		default:
			legacyActual[i] = core.TierMid
		}
		legacyPredicted[i] = r.Predicted.Tier()
	}
	legacyMetrics := training.Compute(legacyActual, legacyPredicted)

	// Print comparison
	fmt.Fprintln(os.Stdout, "Benchmark: Legacy vs Capability Router v2")
	if candidateWeights != "" {
		fmt.Fprintf(os.Stdout, "Candidate weights: %s\n", candidateWeights)
	}
	fmt.Fprintf(os.Stdout, "Dataset: %d tasks\n\n", v2ds.Size())
	fmt.Fprintf(os.Stdout, "Label support: SMALL=%d MID=%d FRONTIER=%d\n", classCounts[core.TierSmall], classCounts[core.TierMid], classCounts[core.TierFrontier])
	if classCounts[core.TierSmall] < 5 || classCounts[core.TierMid] < 5 || classCounts[core.TierFrontier] < 5 {
		fmt.Fprintln(os.Stdout, "Warning: fewer than 5 examples in at least one tier; treat this comparison as exploratory.")
	}

	modelLabel := "v2"
	if candidateWeights != "" {
		modelLabel = "Candidate"
	}
	fmt.Fprintf(os.Stdout, "%-35s %10s %10s %10s\n", "", "Legacy", modelLabel, "Delta")
	fmt.Fprintf(os.Stdout, "%-35s %10.1f%% %10.1f%% %+9.1f%%\n",
		"Tier accuracy",
		legacyMetrics.Accuracy*100, v2Metrics.Accuracy*100,
		(v2Metrics.Accuracy-legacyMetrics.Accuracy)*100)
	fmt.Fprintf(os.Stdout, "%-35s %10.1f%% %10.1f%% %+9.1f%%\n",
		"Unsafe downgrade",
		legacyMetrics.UnsafeDowngradeRate*100, v2Metrics.UnsafeDowngradeRate*100,
		(v2Metrics.UnsafeDowngradeRate-legacyMetrics.UnsafeDowngradeRate)*100)
	fmt.Fprintf(os.Stdout, "  %-33s %10d  %10d\n",
		"FRONTIER → SMALL",
		legacyMetrics.Unsafe.FrontierToSmall, v2Metrics.Unsafe.FrontierToSmall)
	fmt.Fprintf(os.Stdout, "  %-33s %10d  %10d\n",
		"FRONTIER → MID",
		legacyMetrics.Unsafe.FrontierToMid, v2Metrics.Unsafe.FrontierToMid)
	fmt.Fprintf(os.Stdout, "%-35s %10.1f%% %10.1f%% %+9.1f%%\n",
		"Over-routing",
		legacyMetrics.OverRoutingRate*100, v2Metrics.OverRoutingRate*100,
		(v2Metrics.OverRoutingRate-legacyMetrics.OverRoutingRate)*100)
	fmt.Fprintf(os.Stdout, "%-35s %10.3f  %10.3f  %+9.3f\n",
		"Risk-weighted loss",
		legacyMetrics.RiskWeightedLoss, v2Metrics.RiskWeightedLoss,
		v2Metrics.RiskWeightedLoss-legacyMetrics.RiskWeightedLoss)
	fmt.Fprintln(os.Stdout, "")

	// Verdict
	if v2Metrics.Unsafe.FrontierToSmall < legacyMetrics.Unsafe.FrontierToSmall ||
		v2Metrics.UnsafeDowngradeRate < legacyMetrics.UnsafeDowngradeRate {
		fmt.Fprintln(os.Stdout, "Recommendation: Capability v2 reduces unsafe downgrades.")
		fmt.Fprintln(os.Stdout, "                Accept any over-routing increase for safety gain.")
	} else if v2Metrics.Accuracy > legacyMetrics.Accuracy {
		fmt.Fprintln(os.Stdout, "Recommendation: Capability v2 improves overall accuracy.")
	} else {
		fmt.Fprintln(os.Stdout, "Recommendation: No clear improvement — keep legacy as default.")
		fmt.Fprintln(os.Stdout, "                Collect more training data and re-run train + compare.")
	}

	return 0
}

// runFeedback records engineer-reviewed results without coupling the loop to
// any harness completion protocol.
func runFeedback(args []string) int {
	if len(args) > 0 && args[0] == "stats" {
		if len(args) != 1 {
			fmt.Fprintln(os.Stderr, "usage: downshift feedback stats")
			return 2
		}
		store := training.NewEventStore(training.DefaultEventsPath())
		events, err := store.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading loop events: %v\n", err)
			return 1
		}
		type counts struct{ pending, success, retry, failed, trainable int }
		byHarness := make(map[string]counts)
		for _, event := range events {
			c := byHarness[event.Harness]
			if event.Outcome != nil && event.Outcome.RequiredTier != nil {
				c.trainable++
			}
			if event.Outcome == nil {
				c.pending++
			} else {
				switch {
				case event.Outcome.Success:
					c.success++
				case event.Outcome.Retry:
					c.retry++
				case event.Outcome.Failed:
					c.failed++
				}
			}
			byHarness[event.Harness] = c
		}
		if len(events) == 0 {
			fmt.Fprintln(os.Stdout, "No routing events yet.")
			return 0
		}
		fmt.Fprintf(os.Stdout, "%-16s %8s %8s %8s %8s %10s\n", "Harness", "Success", "Retry", "Failed", "Pending", "Trainable")
		var names []string
		for name := range byHarness {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			c := byHarness[name]
			fmt.Fprintf(os.Stdout, "%-16s %8d %8d %8d %8d %10d\n", name, c.success, c.retry, c.failed, c.pending, c.trainable)
		}
		return 0
	}
	if len(args) == 0 || args[0] == "list" {
		pendingOnly := len(args) == 2 && args[1] == "--pending"
		if len(args) > 2 || (len(args) == 2 && !pendingOnly) {
			fmt.Fprintln(os.Stderr, "usage: downshift feedback list [--pending] | stats")
			return 2
		}
		store := training.NewEventStore(training.DefaultEventsPath())
		events, err := store.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading loop events: %v\n", err)
			return 1
		}
		if len(events) == 0 {
			fmt.Fprintln(os.Stdout, "No routing events yet. Run a subagent task through a configured harness first.")
			return 0
		}
		shown := 0
		for i := len(events) - 1; i >= 0 && shown < 50; i-- {
			event := events[i]
			if pendingOnly && event.Outcome != nil {
				continue
			}
			outcome := "pending"
			if event.Outcome != nil {
				switch {
				case event.Outcome.Success:
					outcome = "success"
				case event.Outcome.Retry:
					outcome = "retry → " + event.Outcome.RetryTier.String()
				case event.Outcome.Failed:
					outcome = "failed"
				}
			}
			fmt.Fprintf(os.Stdout, "%s  %-10s %-8s %-14s %s\n", event.ID, event.Harness, event.SelectedTier, outcome, event.Timestamp.Format(time.RFC3339))
			shown++
		}
		return 0
	}
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: downshift feedback list [--pending] | stats | <id> <success|retry|failed> [--retry-tier=MID|FRONTIER] [--required-tier=SMALL|MID|FRONTIER]")
		return 2
	}
	outcome := training.Outcome{}
	switch args[1] {
	case "success":
		outcome.Success = true
	case "retry":
		outcome.Retry = true
	case "failed":
		outcome.Failed = true
	default:
		fmt.Fprintf(os.Stderr, "unknown outcome %q; use success, retry, or failed\n", args[1])
		return 2
	}
	for _, arg := range args[2:] {
		if strings.HasPrefix(arg, "--retry-tier=") {
			switch strings.ToUpper(strings.TrimPrefix(arg, "--retry-tier=")) {
			case "MID":
				outcome.RetryTier = core.TierMid
			case "FRONTIER":
				outcome.RetryTier = core.TierFrontier
			default:
				fmt.Fprintln(os.Stderr, "retry tier must be MID or FRONTIER")
				return 2
			}
		} else if strings.HasPrefix(arg, "--required-tier=") {
			var required core.Tier
			switch strings.ToUpper(strings.TrimPrefix(arg, "--required-tier=")) {
			case "SMALL":
				required = core.TierSmall
			case "MID":
				required = core.TierMid
			case "FRONTIER":
				required = core.TierFrontier
			default:
				fmt.Fprintln(os.Stderr, "required tier must be SMALL, MID, or FRONTIER")
				return 2
			}
			outcome.RequiredTier = &required
		} else {
			fmt.Fprintf(os.Stderr, "unknown feedback option %q\n", arg)
			return 2
		}
	}
	store := training.NewEventStore(training.DefaultEventsPath())
	if err := store.AddOutcome(args[0], outcome); err != nil {
		fmt.Fprintf(os.Stderr, "error recording feedback: %v\n", err)
		return 1
	}
	fmt.Fprintf(os.Stdout, "Feedback recorded for %s. Train a candidate with `downshift train --from-events --output=candidate.json`.\n", args[0])
	return 0
}

func legacyTasksForComparison(examples []training.Example) ([]benchmark.Task, error) {
	labels := map[string]string{
		"SMALL":    "TRIVIAL",
		"MID":      "MEDIUM",
		"FRONTIER": "COMPLEX",
	}
	tasks := make([]benchmark.Task, 0, len(examples))
	for _, example := range examples {
		label, ok := labels[strings.ToUpper(strings.TrimSpace(example.Label))]
		if !ok {
			return nil, fmt.Errorf("unsupported tier %q; use SMALL, MID, or FRONTIER", example.Label)
		}
		tasks = append(tasks, benchmark.Task{Prompt: example.Prompt, Label: label})
	}
	return tasks, nil
}

// runTrain trains the capability router v2 classifier on a labelled dataset.
// Usage: downshift train <dataset.json> [--output=weights.json] [--validation-split=0.2] [--from-events]
func runTrain(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: downshift train <dataset.json> [--output=<path>] [--validation-split=0.2]")
		fmt.Fprintln(os.Stderr, "       downshift train --from-events [--output=<path>]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Dataset format: [{\"prompt\":\"...\",\"label\":\"SMALL|MID|FRONTIER\"}, ...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  --output=<path>          Where to write weights.json (default: ./weights.json)")
		fmt.Fprintln(os.Stderr, "  --validation-split=0.2   Fraction to use for validation (default: 0.2)")
		fmt.Fprintln(os.Stderr, "  --from-events            Train from collected routing events instead of a dataset")
		fmt.Fprintln(os.Stderr, "  --epochs=100             Number of training epochs (default: 100)")
		fmt.Fprintln(os.Stderr, "  --learning-rate=0.1      Learning rate for gradient descent (default: 0.1)")
		return 2
	}

	// Parse flags
	var datasetPath string
	outputPath := "weights.json"
	validationSplit := 0.2
	fromEvents := false
	config := training.DefaultTrainConfig()

	for _, arg := range args {
		switch {
		case arg == "--from-events":
			fromEvents = true
		case len(arg) > 9 && arg[:9] == "--output=":
			outputPath = arg[9:]
		case len(arg) > 19 && arg[:19] == "--validation-split=":
			v, err := strconv.ParseFloat(arg[19:], 64)
			if err == nil && v > 0 && v < 1 {
				validationSplit = v
			}
		case len(arg) > 9 && arg[:9] == "--epochs=":
			v, err := strconv.Atoi(arg[9:])
			if err == nil && v > 0 {
				config.Epochs = v
			}
		case len(arg) > 16 && arg[:16] == "--learning-rate=":
			v, err := strconv.ParseFloat(arg[16:], 64)
			if err == nil && v > 0 {
				config.LearningRate = v
			}
		default:
			if !fromEvents {
				datasetPath = arg
			}
		}
	}

	fmt.Fprintln(os.Stdout, "Training capability-router v2")
	fmt.Fprintln(os.Stdout, "")

	var result training.TrainResult
	var err error

	if fromEvents {
		eventsPath := training.DefaultEventsPath()
		fmt.Fprintf(os.Stdout, "Source:  events file (%s)\n", eventsPath)
		store := training.NewEventStore(eventsPath)
		if _, err = store.Load(); err == nil {
			labeled := store.ToDataset()
			fmt.Fprintf(os.Stdout, "Reviewed minimum-tier labels: %d\n", labeled.Size())
			trainDS, valDS := labeled.Split(validationSplit)
			fmt.Fprintf(os.Stdout, "Train/Val: %d / %d\n\n", trainDS.Size(), valDS.Size())
			result = training.Train(trainDS, valDS, config)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading events: %v\n", err)
			return 1
		}
		if result.Weights == nil {
			fmt.Fprintln(os.Stderr, "no reviewed minimum-tier labels yet — add --required-tier=SMALL|MID|FRONTIER when recording feedback")
			return 1
		}
	} else {
		if datasetPath == "" {
			fmt.Fprintln(os.Stderr, "error: dataset path required (or use --from-events)")
			return 2
		}

		ds, loadErr := training.LoadDataset(datasetPath)
		if loadErr != nil {
			fmt.Fprintf(os.Stderr, "error loading dataset: %v\n", loadErr)
			return 1
		}

		fmt.Fprintf(os.Stdout, "Dataset: %d tasks\n", ds.Size())
		trainDS, valDS := ds.Split(validationSplit)
		fmt.Fprintf(os.Stdout, "Train/Val: %d / %d\n\n", trainDS.Size(), valDS.Size())

		result = training.Train(trainDS, valDS, config)
	}

	// Print training results
	fmt.Fprintf(os.Stdout, "Epochs: %d  LR: %.3f  Risk-weighted: %v\n\n",
		config.Epochs, config.LearningRate, config.RiskWeighted)
	fmt.Fprintf(os.Stdout, "Training metrics:\n%s\n", result.TrainMetrics.Summary())
	if result.ValMetrics.ConfusionMatrix.Total() > 0 {
		fmt.Fprintf(os.Stdout, "Validation metrics:\n%s\n", result.ValMetrics.Summary())
	}

	// Save weights
	if err := classifier.SaveWeights(result.Weights, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "error saving weights: %v\n", err)
		return 1
	}

	fmt.Fprintf(os.Stdout, "Weights saved to: %s\n", outputPath)
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintf(os.Stdout, "To evaluate: downshift benchmark <holdout.json> --compare --candidate-weights=%s\n", outputPath)
	fmt.Fprintln(os.Stdout, "Candidate weights are not activated automatically; review safety and quality metrics before promotion.")
	return 0
}

// --- Harness-specific fail-open responses ---
// Each harness has a different envelope for "allow unchanged". These are the
// minimal valid JSON outputs that let the tool call proceed unmodified.

func printAllow() {
	fmt.Println(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`)
}

func printCursorAllow() {
	fmt.Println(`{"permission":"allow"}`)
}

func printCodexAllow() {
	fmt.Println(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"allow"}}`)
}

func usage(w io.Writer) {
	fmt.Fprint(w, `downshift — right-sized models for every subagent task

Usage:
  downshift antigravity          Run as an Antigravity PreToolUse hook
  downshift claude-code          Run as a Claude Code PreToolUse hook (reads stdin)
  downshift cursor               Run as a Cursor preToolUse hook (reads stdin)
  downshift codex                Run as a Codex PreToolUse hook (reads stdin)
  downshift kirocrew             Run as a KiroCrew preToolUse hook (policy mode: exit 0/2)
  downshift serve                Start the web dashboard at http://localhost:7474 (serves web/)
  downshift try "<task>" [harness] [model]   Test classification from the terminal
  downshift version              Print the binary version (also --version)
  downshift models list          Show the effective catalog (embedded or override)
  downshift models check         Query provider APIs and report new/untiered models
  downshift doctor               Print version, state dir, and catalog source
  downshift models pull          Write user catalog.json under the state dir
  downshift stats [--days=N]     Show routing decisions and estimated savings (default: 30 days)
  downshift stats --cost-per-unit=<USD>   Convert normalised units to dollars
  downshift stats --export       Print a pasteable JSON summary without prompts
  downshift benchmark <file>     Run classifier against a labelled dataset; print confusion matrix
  downshift benchmark <file> --compare  Compare Legacy vs CapabilityRouter v2 side by side
  downshift eval-outcome --verify|--report  Outcome eval: executable checks, small vs frontier pass rate
  downshift train <file>         Train capability-router v2 on a labelled dataset
  downshift train --from-events  Train from engineer-reviewed local feedback
  downshift shadow-report       Compare opt-in candidate observations with explicit reviewed labels (JSON)
  downshift feedback list        List routing IDs awaiting engineer review
  downshift feedback stats       Summarize outcomes per harness
  downshift feedback <id> <outcome>  Record success, retry, or failed

Monitor (separate binary — cmd/dsmon):
  go build -o dsmon ./cmd/dsmon  Build the live terminal widget
  ./cmd/dsmon/launch.sh          Open dsmon in a floating terminal window

Grok note:
  Grok routes subagent models via config, not a hook (its PreToolUse is
  allow/deny only). Run  downshift try "<task>" grok  to see the reasoning
  effort to pin on a subagent role in ~/.grok/config.toml. See the README.

Examples:
  downshift try "rename the variable userId"
  downshift try "rearchitect the payment flow" cursor claude-haiku-5-5
  downshift try "add a subagent to scan for secrets" codex gpt-5.6-sol
  downshift try "explore the auth module" grok
  downshift stats
  downshift stats --days=7
  downshift benchmark path/to/tasks.json
`)
}

// runClaudeSubagentStop is the SubagentStop hook runner for Claude Code. The
// subagent already finished, so it never blocks: any failure exits 0 with no
// output, and usage goes to the hermetic event log.
func runClaudeSubagentStop(in io.Reader, catalog core.Resolver) (rc int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "downshift: internal error in SubagentStop, ignored")
			rc = 0
		}
	}()
	const maxHookPayloadBytes = 1 << 20
	data, err := readHookPayload(in, maxHookPayloadBytes+1)
	if err != nil || len(data) > maxHookPayloadBytes {
		return 0
	}
	if note := claudecode.HandleSubagentStop(data, buildVersion, catalog); note != "" {
		fmt.Fprintf(os.Stderr, "downshift: %s\n", note)
	}
	return 0
}
