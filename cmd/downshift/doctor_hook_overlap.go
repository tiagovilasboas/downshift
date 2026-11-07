// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/setup"
)

// Hook overlap check for `doctor`.
//
// Two hooks on the same event and the same tool can both answer with a rewrite
// of the tool input (an updated input). The harness applies one of them; the
// other is lost with no error and no log line on either side. Downshift cannot
// read what another hook prints, so the check works from the config alone: it
// reports a conflict only when the other hook is a recognised rewriter of the
// same kind (model or shell command) on an overlapping matcher. An unrecognised
// hook on the same tool is mentioned in --verbose only, because most hooks on a
// tool (audit, guard, log) never rewrite.
//
// Nothing here names a harness. What each Downshift hook rewrites is data in
// setup.Targets (Hook.Rewrites); the event, matcher and file come from the same
// table, so every harness whose hook file doctor already reads is covered.
// Evidence and the measured winner rule: docs/guides/companions.md,
// "Hook coexistence", and docs/evidence/hook-coexistence-2026-11-06.md.

// hookEntry is one command found in a hook file, with the keys above it, the
// matcher of its group, and the argument list of an exec-form entry.
type hookEntry struct {
	Path    []string
	Matcher string
	Command string
	Args    []string
}

// collectHookEntries walks a parsed hook file. A "matcher" on an object applies
// to every command below it (nested groups) and to its own "command" (flat
// entries), so both entry shapes yield the same hookEntry.
func collectHookEntries(v any, path []string, matcher string, out *[]hookEntry) {
	switch t := v.(type) {
	case map[string]any:
		if m, ok := t["matcher"].(string); ok {
			matcher = m
		}
		if cmd, ok := t["command"].(string); ok {
			e := hookEntry{Path: path, Matcher: matcher, Command: cmd}
			if args, ok := t["args"].([]any); ok {
				for _, a := range args {
					if s, ok := a.(string); ok {
						e.Args = append(e.Args, s)
					}
				}
			}
			*out = append(*out, e)
		}
		for k, child := range t {
			if k == "command" || k == "args" {
				continue
			}
			collectHookEntries(child, append(append([]string(nil), path...), k), matcher, out)
		}
	case []any:
		for _, child := range t {
			collectHookEntries(child, path, matcher, out)
		}
	}
}

// knownRewriter recognises a third-party hook that rewrites tool input.
// Downshift neither installs nor wraps these (docs/guides/companions.md); the
// table only lets doctor say what the config shows.
type knownRewriter struct {
	Name     string
	Rewrites string // setup.RewritesModel or setup.RewritesShell
	// Is reports whether a launch candidate (see launchCandidates) is the tool.
	Is func(base string) bool
}

var knownHookRewriters = []knownRewriter{
	// RTK (rtk-ai/rtk) rewrites the shell command: `rtk hook <agent>` or the
	// rtk-rewrite script it installs.
	{Name: "RTK", Rewrites: setup.RewritesShell, Is: func(b string) bool {
		return b == "rtk" || strings.HasPrefix(b, "rtk-rewrite")
	}},
	// cheap-subagents (The-Guard305/cheap-subagents) sets the subagent model.
	{Name: "cheap-subagents", Rewrites: setup.RewritesModel, Is: func(b string) bool {
		return b == "cheap_subagents"
	}},
}

// interpreters are launchers whose first non-flag argument is the real hook.
var interpreters = map[string]bool{
	"bash": true, "sh": true, "zsh": true, "env": true, "node": true,
	"python": true, "python3": true, "py": true, "powershell": true, "pwsh": true, "cmd": true,
}

// normBase is the lower-case file name of a command token without a launcher
// extension, with either path separator accepted.
func normBase(tok string) string {
	b := strings.ToLower(path.Base(strings.ReplaceAll(strings.Trim(tok, `"'`), `\`, "/")))
	for _, ext := range []string{".exe", ".cmd", ".sh", ".ps1", ".py", ".js", ".mjs"} {
		b = strings.TrimSuffix(b, ext)
	}
	return b
}

// commandWords splits a hook command into words. A quoted first word may hold
// spaces (`"C:\Program Files\rtk\rtk.exe" hook claude`), as in
// setup.IsDownshiftCommand; the rest is split on white space.
func commandWords(command string) []string {
	c := strings.TrimSpace(command)
	if strings.HasPrefix(c, `"`) {
		if end := strings.Index(c[1:], `"`); end >= 0 {
			return append([]string{c[1 : 1+end]}, strings.Fields(c[2+end:])...)
		}
	}
	return strings.Fields(c)
}

// launchCandidates returns the names that identify what a hook entry runs: the
// executable and, behind an interpreter, the script it is given.
func launchCandidates(e hookEntry) []string {
	toks := append(commandWords(e.Command), e.Args...)
	if len(toks) == 0 {
		return nil
	}
	out := []string{normBase(toks[0])}
	if interpreters[out[0]] {
		// `env python3 hook.py`, `bash -e hook.sh`: the hook is the first
		// argument that is neither a flag nor another launcher.
		for _, t := range toks[1:] {
			if strings.HasPrefix(t, "-") {
				continue
			}
			b := normBase(t)
			out = append(out, b)
			if !interpreters[b] {
				break
			}
		}
	}
	return out
}

// recogniseRewriter returns the known rewriter e runs, if any.
func recogniseRewriter(e hookEntry) (knownRewriter, bool) {
	for _, c := range launchCandidates(e) {
		for _, k := range knownHookRewriters {
			if k.Is(c) {
				return k, true
			}
		}
	}
	return knownRewriter{}, false
}

// overlapFix says how to keep a single rewriter, by what is rewritten.
func overlapFix(rewrites, other, event, file string) string {
	entry := fmt.Sprintf("remove the %s entry under %q in %s", other, event, file)
	switch rewrites {
	case setup.RewritesShell:
		return "Keep one shell rewriter: set DOWNSHIFT_EXEC=0 (or \"exec\": false in context-optimization.json) to leave the command to " +
			other + ", or " + entry
	default:
		return "Keep one model chooser: " + entry + " (Downshift applies the orchestrator's model choice within your declared policy)"
	}
}

// checkHookOverlap adds a "hook overlap" warning for every recognised rewriter
// of the same kind that shares an event and a tool with an active Downshift
// rewriting hook. A Downshift hook is active when its entry is in the file (the
// matcher written there is the one that counts, since setup keeps a matcher the
// user already had) or when the plugin carries it (the shipped matcher counts).
// matchWiredHooks pairs hooks by command alone, so the matcher is checked here.
// With verbose it also counts the unrecognised hooks on those tools. execOn is
// whether the compact wrapper is on: with it off Downshift rewrites no shell
// command, so a shell rewriter beside it is not an overlap (that is one of the
// two fixes the warning offers).
func checkHookOverlap(add func(id, status, detail, fix string), verbose bool, w doctorWiring, file string, raw any, viaPlugin, execOn bool) {
	var entries []hookEntry
	collectHookEntries(raw, nil, "", &entries)

	isDownshift := func(e hookEntry) bool {
		for _, h := range w.Hooks {
			if _, ok := matchHook(e.Command, h); ok {
				return true
			}
		}
		return false
	}

	// rewriter is an active Downshift hook that rewrites input, with the matcher
	// that decides which tools it answers.
	type rewriter struct {
		spec    doctorHook
		matcher string
	}
	var active []rewriter
	for _, spec := range w.Hooks {
		if spec.Rewrites == "" || (spec.Rewrites == setup.RewritesShell && !execOn) {
			continue
		}
		if viaPlugin {
			active = append(active, rewriter{spec, spec.Matcher})
			continue
		}
		for _, e := range entries {
			if _, ok := matchHook(e.Command, spec); ok && underEvent(e.Path, spec.Event) && matchersOverlap(spec.Matcher, e.Matcher) {
				active = append(active, rewriter{spec, e.Matcher})
			}
		}
	}

	type pair struct{ name, rewrites, event string }
	reported := map[pair]bool{}
	unknown := map[int]bool{} // entries with no recognised tool, counted once each
	for _, r := range active {
		for i, e := range entries {
			if !underEvent(e.Path, r.spec.Event) || isDownshift(e) || !matchersOverlap(r.matcher, e.Matcher) {
				continue
			}
			k, known := recogniseRewriter(e)
			if !known {
				unknown[i] = true
				continue
			}
			key := pair{k.Name, r.spec.Rewrites, r.spec.Event}
			if k.Rewrites != r.spec.Rewrites || reported[key] {
				continue
			}
			reported[key] = true
			what := map[string]string{setup.RewritesModel: "model", setup.RewritesShell: "command"}[r.spec.Rewrites]
			// The detail stays short: text mode cuts a row to the terminal width,
			// and the fix line is printed whole.
			add("hook overlap", doctorWarn,
				fmt.Sprintf("%s also rewrites the %s; one rewrite is lost", k.Name, what),
				fmt.Sprintf("%s (matcher %q) and Downshift (matcher %q) both answer the same %s call; the harness does not merge the rewrites and does not guarantee which one runs (measured where the harness runs hooks in parallel: the hook that finishes last, not the first in the file; docs/guides/companions.md, Hook coexistence), and the other is lost with no error. %s",
					k.Name, e.Matcher, r.matcher, r.spec.Event, overlapFix(r.spec.Rewrites, k.Name, r.spec.Event, file)))
		}
	}
	if verbose && len(unknown) > 0 && len(reported) == 0 {
		add("hook overlap", doctorOK,
			fmt.Sprintf("%d other hook(s) in %s share an event and tool with a Downshift rewriting hook; the config cannot show whether they rewrite the input", len(unknown), file), "")
	}
}
