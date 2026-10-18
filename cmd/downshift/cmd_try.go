// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/catalog"
	"github.com/tiagovilasboas/downshift/internal/core"
)

type catalogEntryReader interface {
	Entries() []catalog.Entry
}

// runTry classifies a prompt from the command line for quick testing.
func runTry(resolver core.Resolver, args []string) int {
	args, detail := stripDetail(args)
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
	if harnessRegistry[harness].Caps.SingleModelReasoning {
		cls := core.Classify(prompt)
		tier := core.TierOf(cls.Complexity)
		effort := core.EffortFor(tier)
		fmt.Printf("Task:       %s\n", prompt)
		fmt.Printf("Complexity: %s\n", cls.Complexity)
		fmt.Printf("Needs tier: %s\n", tier)
		fmt.Printf("Reasoning:  %s  (grok-4.6, configurable reasoning)\n", effort)
		fmt.Printf("→ set reasoning_effort=%q on the subagent role/persona in config.toml\n", effort.String())
		return 0
	}

	return runTryReport(resolver, harness, prompt, current, detail)
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
	names := map[string]struct{}{}
	for _, id := range harnessIDs() {
		names[id] = struct{}{}
	}
	for _, c := range core.AllHarnessCaps() {
		names[c.Harness] = struct{}{}
	}
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
	e, ok := harnessRegistry[harness]
	if !ok || e.Try == nil {
		return tryResult{}, false
	}
	return e.Try(prompt, current, catalog), true
}
