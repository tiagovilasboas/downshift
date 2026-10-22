// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"io"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// genericPromptHarness is the one id accepted by the prompt-context runner
// that is not a registered harness: a harness-agnostic caller of the DCI hook.
const genericPromptHarness = "generic"

// promptContextSource turns one submitted prompt into extra model context. It
// returns "" to abstain; the runner then stays silent. It owns the domain
// (what to retrieve); the runner owns the harness protocol.
type promptContextSource func(prompt, cwd string) string

// promptContextHarness reports whether the harness declares it can take prompt
// context (HarnessCapabilities.CanInjectPromptContext).
func promptContextHarness(harness string) bool {
	if harness == genericPromptHarness {
		return true
	}
	caps, ok := core.CapsFor(harness)
	return ok && caps.CanInjectPromptContext
}

// runPromptContext is the shared runner of the prompt-submit hook. It reads
// one event from in, asks source for context and writes the harness response
// to w. Unlike runHookAdapter it routes no spawn, so it has no control group,
// pause or telemetry of its own, and fail-open is silence: an unsupported
// harness, an unreadable or invalid event, an abstention or a panic all exit 0
// with nothing on stdout.
func runPromptContext(harness string, in io.Reader, w io.Writer, source promptContextSource) int {
	defer func() {
		// Recover from unexpected panics to guarantee fail-open.
		_ = recover()
	}()
	if !promptContextHarness(harness) || in == nil {
		return 0
	}
	data, err := io.ReadAll(io.LimitReader(in, 1<<20))
	if err != nil || len(data) == 0 {
		return 0
	}
	var input userPromptSubmitInput
	if err := json.Unmarshal(data, &input); err != nil {
		return 0
	}
	prompt := input.PromptText()
	if prompt == "" {
		return 0
	}
	text := source(prompt, input.Cwd)
	if text == "" {
		return 0
	}
	out := userPromptSubmitOutput{
		HookSpecificOutput: hookSpecificOutput{HookEventName: "UserPromptSubmit", AdditionalContext: text},
	}
	if harness != "claude-code" {
		out.AdditionalContext = text
	}
	_ = json.NewEncoder(w).Encode(out)
	return 0
}
