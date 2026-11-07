// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
)

// Two PreToolUse hooks that both rewrite the same tool input race. Claude Code
// runs the matching hooks in parallel and applies the answer of the one that
// finishes last, whatever the order in the settings file (measured on Claude
// Code 2.1.319, docs/evidence/hook-coexistence-2026-11-07.md). Neither hook is
// told, so a rewrite is dropped without a trace on some calls. Doctor cannot
// read what an arbitrary hook does, so it knows a short list of products whose
// hooks rewrite input. Harness facts stay data: the matchers come from
// setup.Targets through doctorWiring, and the check runs only where the
// harness can rewrite at all (HarnessCapabilities).

// inputRewriter is a product whose PreToolUse hook rewrites the tool input.
type inputRewriter struct {
	Name string
	// Kind is "shell" for a command rewrite (the one Downshift's exec wrapper
	// makes) and "model" for a subagent model rewrite.
	Kind    string
	Command *regexp.Regexp // matched against the hook command and its args
}

var knownRewriters = []inputRewriter{
	{Name: "RTK", Kind: "shell", Command: regexp.MustCompile(`(^|[/\\ ])rtk(\.exe)?\s+hook\b|rtk-rewrite`)},
	{Name: "cheap-subagents", Kind: "model", Command: regexp.MustCompile(`cheap_subagents`)},
}

// hookGroup is one entry of an event: its matcher and the commands it runs.
type hookGroup struct {
	Matcher  string
	Commands []string
}

// hookGroups reads the groups registered under event in a hook file, in either
// entry shape: {"matcher", "hooks":[{"command","args"}]} or {"matcher","command"}.
func hookGroups(raw any, event string) []hookGroup {
	doc, _ := raw.(map[string]any)
	events, _ := doc["hooks"].(map[string]any)
	var out []hookGroup
	for name, list := range events {
		if !strings.EqualFold(name, event) {
			continue
		}
		entries, _ := list.([]any)
		for _, e := range entries {
			entry, _ := e.(map[string]any)
			g := hookGroup{Matcher: fmt.Sprint(valueOr(entry["matcher"], ""))}
			g.Commands = append(g.Commands, commandLine(entry))
			inner, _ := entry["hooks"].([]any)
			for _, h := range inner {
				hm, _ := h.(map[string]any)
				g.Commands = append(g.Commands, commandLine(hm))
			}
			out = append(out, g)
		}
	}
	return out
}

func valueOr(v any, def any) any {
	if v == nil {
		return def
	}
	return v
}

// commandLine joins a hook's command and args into one string; "" when none.
func commandLine(m map[string]any) string {
	cmd, ok := m["command"].(string)
	if !ok {
		return ""
	}
	parts := []string{cmd}
	if args, ok := m["args"].([]any); ok {
		for _, a := range args {
			parts = append(parts, fmt.Sprint(a))
		}
	}
	return strings.Join(parts, " ")
}

var plainMatcher = regexp.MustCompile(`^[A-Za-z0-9_|]*$`)

// matcherTokens lists the literal names a matcher spells out.
func matcherTokens(m string) []string {
	var out []string
	for _, t := range strings.Split(m, "|") {
		t = strings.Trim(strings.TrimSpace(t), "^$()")
		if t != "" && plainMatcher.MatchString(t) {
			out = append(out, t)
		}
	}
	return out
}

// matcherCovers reports whether matcher m selects the tool name t. An empty or
// "*" matcher selects every tool; a plain list of names is exact; anything else
// is a regular expression.
func matcherCovers(m, t string) bool {
	if m == "" || m == "*" {
		return true
	}
	if plainMatcher.MatchString(m) {
		for _, n := range strings.Split(m, "|") {
			if n == t {
				return true
			}
		}
		return false
	}
	re, err := regexp.Compile(m)
	return err == nil && re.MatchString(t)
}

// sharedTools lists the tool names two matchers both select, among the names
// either one spells out.
func sharedTools(a, b string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range append(matcherTokens(a), matcherTokens(b)...) {
		if !seen[t] && matcherCovers(a, t) && matcherCovers(b, t) {
			seen[t] = true
			out = append(out, t)
		}
	}
	return out
}

// checkRewriteConflicts warns when a known input-rewriting hook sits on the
// same event and tool as a Downshift hook that rewrites. execOn is whether the
// compact wrapper is on: with it off Downshift rewrites no shell command, so a
// shell rewriter next to it is no longer a conflict.
func checkRewriteConflicts(add func(id, status, detail, fix string), nc core.NamedCaps, w doctorWiring, raw any, file string, execOn bool) {
	if !nc.Caps.CanRewriteModel && !nc.Caps.CanRewriteShellCommand {
		return
	}
	for _, own := range w.Hooks {
		// The hooks that rewrite run the subcommand named after the harness.
		if own.Args != nc.Harness {
			continue
		}
		for _, g := range hookGroups(raw, own.Event) {
			for _, c := range g.Commands {
				for _, rw := range knownRewriters {
					if c == "" || !rw.Command.MatchString(c) || (rw.Kind == "shell" && !execOn) {
						continue
					}
					shared := sharedTools(own.Matcher, g.Matcher)
					if len(shared) == 0 {
						continue
					}
					add("hook conflict", doctorWarn, fmt.Sprintf(
						"%s also rewrites %s input on %s in %s: when two hooks rewrite the same input only one rewrite is applied, and which one is not fixed (on Claude Code the hook that finishes last wins), so Downshift's rewrite is dropped on some calls",
						rw.Name, strings.Join(shared, "/"), own.Event, file), conflictFix(rw, file))
				}
			}
		}
	}
}

func conflictFix(rw inputRewriter, file string) string {
	if rw.Kind == "shell" {
		return fmt.Sprintf(`keep one rewriter per tool: remove the %s hook from %s, or turn Downshift's command rewrite off with "exec": false in context-optimization.json (DOWNSHIFT_EXEC=0)`, rw.Name, file)
	}
	return fmt.Sprintf("remove the %s hook from %s: Downshift already applies the orchestrator's model choice inside your policy", rw.Name, file)
}
