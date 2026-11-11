// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
	"github.com/tiagovilasboas/downshift-ai/internal/telemetry"
)

// Per-harness hook checks for `doctor`. Harness facts are data: the expected
// hooks come from setup.Targets, the table `setup` writes from. Nothing here
// names a harness.

// doctorHook is one expected hook.
type doctorHook struct {
	Event   string
	Matcher string
	Args    string // subcommand and arguments after the binary
	Wrapper string // stem of a script that carries the subcommand (setup.Hook.Wrapper)
	// Rewrites is setup.Hook.Rewrites: what the hook changes in the input it answers.
	Rewrites string
	// Required: routing does not work without it. The first hook of a harness
	// is the routing hook; the rest only observe (honor, usage).
	Required bool
}

type doctorWiring struct {
	File   string // relative to HOME
	Nested bool   // entry shape, see setup.Target.Nested
	Setup  bool   // `downshift setup <harness>` can write the file
	Plugin string // plugin id that can carry the same hooks (setup.Target.Plugin)
	// Registry is an informational hook list (setup.Target.Registry). A Downshift
	// entry there duplicates the policy hook in File.
	Registry string
	Hooks    []doctorHook
	// FeatureFlags and Activate: what the harness needs before its hooks run
	// (setup.Target.FeatureFlags, setup.Target.Activate).
	FeatureFlags []setup.FeatureFlag
	Activate     string
}

func doctorWiringFor(harness string) (doctorWiring, bool) {
	for _, t := range setup.Targets {
		if t.Name != harness || t.File == "" {
			continue
		}
		w := doctorWiring{File: t.File, Nested: t.Nested, Setup: true, Plugin: t.Plugin, Registry: t.Registry,
			FeatureFlags: t.FeatureFlags, Activate: t.Activate}
		for i, h := range t.Hooks {
			w.Hooks = append(w.Hooks, doctorHook{Event: h.Event, Matcher: h.Matcher, Args: h.Args, Wrapper: h.Wrapper, Rewrites: h.Rewrites, Required: i == 0})
		}
		return w, true
	}
	return doctorWiring{}, false
}

// foundCommand is a "command" string in a hook file and the keys above it.
type foundCommand struct {
	Path    []string
	Command string
}

func collectCommands(v any, path []string, out *[]foundCommand) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			if s, ok := child.(string); ok && k == "command" {
				*out = append(*out, foundCommand{Path: path, Command: s})
				continue
			}
			collectCommands(child, append(append([]string(nil), path...), k), out)
		}
	case []any:
		for _, child := range t {
			collectCommands(child, path, out)
		}
	}
}

func underEvent(path []string, event string) bool {
	for _, k := range path {
		if strings.EqualFold(k, event) {
			return true
		}
	}
	return false
}

// matchHook reports whether command is the expected hook, and the binary token
// it runs ("" for a wrapper script).
func matchHook(command string, h doctorHook) (bin string, ok bool) {
	tok := strings.Fields(command)
	if len(tok) == 0 {
		return "", false
	}
	base := filepath.Base(tok[0])
	if h.Wrapper != "" && setup.IsWrapperCommand(command, h.Wrapper) {
		return "", true
	}
	if base != "downshift" && base != "downshift.exe" {
		return "", false
	}
	if strings.Join(tok[1:], " ") != h.Args {
		return "", false
	}
	return tok[0], true
}

type wiredHook struct {
	Spec    doctorHook
	Bin     string
	Command string
}

func hookSnippet(w doctorWiring, h doctorHook, command string) string {
	if w.Nested {
		inner := fmt.Sprintf(`{"type":"command","command":%q}`, command)
		if h.Matcher != "" {
			return fmt.Sprintf(`{"matcher":%q,"hooks":[%s]}`, h.Matcher, inner)
		}
		return fmt.Sprintf(`{"hooks":[%s]}`, inner)
	}
	if h.Matcher != "" {
		return fmt.Sprintf(`{"matcher":%q,"command":%q}`, h.Matcher, command)
	}
	return fmt.Sprintf(`{"command":%q}`, command)
}

// checkHarness is the orchestrator: read the hook file, report how the hooks
// are wired, then the evidence that they run.
func checkHarness(rep *doctorReport, env doctorEnv, nc core.NamedCaps, w doctorWiring, pathBin string, events []telemetry.Event) {
	h := nc.Harness
	file := filepath.Join(env.Home, w.File)
	add := func(id, status, detail, fix string) {
		rep.add(doctorCheck{ID: id, Harness: h, Status: status, Detail: detail, Fix: fix})
	}

	raw, ok := readHookFile(add, file)
	if !ok {
		return
	}
	var cmds []foundCommand
	collectCommands(raw, nil, &cmds)
	checkMainPolicy(add, h, raw)
	checkModelForce(add, nc.Caps, raw, file, env.Getenv)
	execOn := env.Getenv(execEnvKillSwitch) != "0" && execConfigEnabled()
	viaPlugin := w.Plugin != "" && setup.PluginEnabled(raw, w.Plugin)

	found, missing := matchWiredHooks(cmds, w.Hooks)
	reportWiring(add, h, w, file, pathBin, viaPlugin, found, missing)
	checkInstalledPluginHooks(add, env.Home, h, viaPlugin) // reads the plugin's own hooks.json
	checkHookOverlap(add, env.Verbose, w, file, raw, viaPlugin, execOn)
	// With the plugin carrying the hooks, found is empty: no hook binary to check.
	checkFeatureFlags(add, env.Home, w)
	checkEvidence(add, env, nc, file, found, pathBin, events, activationFix(env.Home, w))
	checkCrewRegistry(add, env, h, w)
	checkHarnessLimitations(add, h)
}
