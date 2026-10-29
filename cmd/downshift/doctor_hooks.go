// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/telemetry"
)

// Per-harness hook checks for `doctor`. Harness facts are data: the expected
// hooks come from setupTargets, the table `setup` writes from. Nothing here
// names a harness.

// doctorHook is one expected hook.
type doctorHook struct {
	Event   string
	Matcher string
	Args    string // subcommand and arguments after the binary
	Wrapper string // stem of a script that carries the subcommand (setupHook.Wrapper)
	// Required: routing does not work without it. The first hook of a harness
	// is the routing hook; the rest only observe (honor, usage).
	Required bool
}

type doctorWiring struct {
	File   string // relative to HOME
	Nested bool   // entry shape, see setupTarget.Nested
	Setup  bool   // `downshift setup <harness>` can write the file
	Plugin string // plugin id that can carry the same hooks (setupTarget.Plugin)
	// ModConflicts: plugin ids whose mod the Plugin already bundles (setupTarget.ModConflicts).
	ModConflicts []string
	// Registry is an informational hook list (setupTarget.Registry). A Downshift
	// entry there duplicates the policy hook in File.
	Registry string
	Hooks    []doctorHook
}

func doctorWiringFor(harness string) (doctorWiring, bool) {
	for _, t := range setupTargets {
		if t.Name != harness || t.File == "" {
			continue
		}
		w := doctorWiring{File: t.File, Nested: t.Nested, Setup: true, Plugin: t.Plugin, ModConflicts: t.ModConflicts, Registry: t.Registry}
		for i, h := range t.Hooks {
			w.Hooks = append(w.Hooks, doctorHook{Event: h.Event, Matcher: h.Matcher, Args: h.Args, Wrapper: h.Wrapper, Required: i == 0})
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
	if h.Wrapper != "" && isWrapperCommand(command, h.Wrapper) {
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
	viaPlugin := w.Plugin != "" && pluginEnabled(raw, w.Plugin)

	found, missing := matchWiredHooks(cmds, w.Hooks)
	reportWiring(add, h, w, file, pathBin, viaPlugin, found, missing)
	reportModConflicts(add, w, raw, viaPlugin)
	// With the plugin carrying the hooks, found is empty and the hook binary
	// check has nothing to look at.
	checkEvidence(add, env, nc, file, found, pathBin, events)
	checkCrewRegistry(add, env, h, w)
	checkHarnessLimitations(add, h)
}

// readHookFile reads and parses the harness hook file. On failure it adds the
// "hooks wired" failure row and reports ok=false.
func readHookFile(add func(id, status, detail, fix string), file string) (raw any, ok bool) {
	data, err := os.ReadFile(file)
	if err == nil {
		err = json.Unmarshal(data, &raw)
	}
	if err != nil {
		add("hooks wired", doctorFail, fmt.Sprintf("cannot read %s: %v", file, err), "python3 -m json.tool "+file)
		return nil, false
	}
	return raw, true
}

// matchWiredHooks pairs each expected hook with the first command under its
// event that runs it. Specs with no match are returned as missing.
func matchWiredHooks(cmds []foundCommand, specs []doctorHook) (found []wiredHook, missing []doctorHook) {
	for _, spec := range specs {
		hit := false
		for _, c := range cmds {
			if !underEvent(c.Path, spec.Event) {
				continue
			}
			if bin, ok := matchHook(c.Command, spec); ok {
				found = append(found, wiredHook{Spec: spec, Bin: bin, Command: c.Command})
				hit = true
				break
			}
		}
		if !hit {
			missing = append(missing, spec)
		}
	}
	return found, missing
}

// reportWiring adds the plugin rows (when the plugin is enabled) and the
// "hooks wired" row, with its fix lines.
func reportWiring(add func(id, status, detail, fix string), h string, w doctorWiring, file, pathBin string, viaPlugin bool, found []wiredHook, missing []doctorHook) {
	// The plugin carries the same hooks. With it enabled, nothing in the
	// settings file is needed; anything found there runs a second time.
	if viaPlugin {
		switch {
		case len(found) > 0:
			add("plugin", doctorWarn,
				fmt.Sprintf("%s is enabled and %d of its hooks are also in %s: every spawn is routed twice", w.Plugin, len(found), file),
				"downshift setup --undo   # keep the plugin; or /plugin uninstall "+w.Plugin+" then downshift setup to keep settings hooks")
		default:
			add("plugin", doctorOK, w.Plugin+" enabled; hooks come from the plugin, none needed in "+file, "")
		}
		if len(found) == 0 {
			add("hooks wired", doctorOK, fmt.Sprintf("%d via plugin %s", len(w.Hooks), w.Plugin), "")
			return
		}
	}
	status, requiredMissing := doctorOK, false
	for _, m := range missing {
		if m.Required {
			requiredMissing = true
		}
	}
	switch {
	case requiredMissing:
		status = doctorFail
	case len(missing) > 0:
		status = doctorWarn
	}
	var have, fixes []string
	for _, f := range found {
		have = append(have, f.Spec.Event+" "+f.Spec.Args)
	}
	binForFix := "downshift"
	if pathBin != "" {
		binForFix = pathBin
	}
	for _, m := range missing {
		command := binForFix + " " + m.Args
		fixes = append(fixes, fmt.Sprintf("add to %s under %q: %s", file, m.Event, hookSnippet(w, m, command)))
	}
	detail := fmt.Sprintf("%d of %d expected (%s)", len(found), len(w.Hooks), strings.Join(have, ", "))
	if len(found) == 0 {
		detail = fmt.Sprintf("0 of %d expected in %s", len(w.Hooks), file)
	}
	if len(missing) > 0 {
		var names []string
		for _, m := range missing {
			names = append(names, m.Event+" "+m.Args)
		}
		detail += "; missing: " + strings.Join(names, ", ")
	}
	if w.Setup && len(missing) > 0 {
		fixes = []string{"downshift setup " + h}
	}
	add("hooks wired", status, detail, strings.Join(fixes, "\n"))
}

// checkEvidence runs the checks that show the wired hooks work: the hook
// binary, the session list, a held quota snapshot, and the last event,
// completion and honored rewrites from the shared event log.
func checkEvidence(add func(id, status, detail, fix string), env doctorEnv, nc core.NamedCaps, file string, found []wiredHook, pathBin string, events []telemetry.Event) {
	h := nc.Harness
	checkHookBinary(add, env, file, found, pathBin)
	checkSessionList(add, env, nc)
	checkAvailableModels(add, env, h)
	checkQuotaHold(add, env, h)
	checkQuotaSource(add, env, h)

	var mine []telemetry.Event
	for _, ev := range events {
		if ev.Harness == h {
			mine = append(mine, ev)
		}
	}
	checkLastEvent(add, env, h, mine)
	checkCompletion(add, h, events)
	if nc.Caps.CanRewriteModel && len(mine) > 0 {
		checkHonored(add, mine)
	}
	checkDuplicateEntries(add, nc, mine)
}

// checkDuplicateEntries warns when the same tool_use_hash appears on more than
// one decision. That shape means two entry points routed one spawn. Only
// harnesses that declare more than one HookEntries are checked; the fix is
// harness-free ("disable the second entry").
func checkDuplicateEntries(add func(id, status, detail, fix string), nc core.NamedCaps, mine []telemetry.Event) {
	if len(nc.Caps.HookEntries) < 2 {
		return
	}
	n := telemetry.DuplicateToolUseCount(mine)
	if n == 0 {
		return
	}
	add("entry dual route", doctorWarn,
		fmt.Sprintf("%d tool_use_hash value(s) have decisions from more than one entry: every spawn is routed twice", n),
		"disable the second entry (settings/plugin PreToolUse or the mod) so only one path is live")
}

// reportModConflicts warns when a plugin whose mod the Downshift plugin already
// bundles is enabled next to it: the mod's hooks and commands then run twice.
// Nothing is reported when the Downshift plugin is off (the bundled mod does
// not load) or no conflict is enabled.
func reportModConflicts(add func(id, status, detail, fix string), w doctorWiring, raw any, viaPlugin bool) {
	if !viaPlugin {
		return
	}
	for _, id := range w.ModConflicts {
		if !pluginEnabled(raw, id) {
			continue
		}
		add("plugin mod", doctorWarn,
			fmt.Sprintf("%s is enabled next to %s, which bundles the same mod: its guard and commands run twice", id, w.Plugin),
			"/plugin uninstall "+id+"   # the Downshift plugin keeps the mod")
	}
}

// pluginEnabled reads the documented `enabledPlugins` map of a settings file:
// {"enabledPlugins": {"name@marketplace": true}}. Only this file is read;
// project-level settings can also enable a plugin and are not checked here.
func pluginEnabled(raw any, id string) bool {
	root, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	m, ok := root["enabledPlugins"].(map[string]any)
	if !ok {
		return false
	}
	v, ok := m[id].(bool)
	return ok && v
}

func checkHookBinary(add func(id, status, detail, fix string), env doctorEnv, file string, found []wiredHook, pathBin string) {
	seen := map[string]bool{}
	var bad, fixes []string
	var good string
	for _, f := range found {
		if f.Bin == "" || seen[f.Bin] {
			continue // a wrapper script carries its own binary
		}
		seen[f.Bin] = true
		bin, err := f.Bin, error(nil)
		if !strings.Contains(bin, "/") {
			bin, err = env.LookPath(bin)
		}
		problem := ""
		switch {
		case err != nil:
			problem = f.Bin + " (not found)"
		case !fileExists(bin):
			problem = bin + " (does not exist)"
		case pathBin != "" && resolvedPath(bin) != resolvedPath(pathBin):
			problem = bin
		}
		if problem == "" {
			good = bin
			continue
		}
		bad = append(bad, problem)
		if pathBin != "" {
			fixes = append(fixes, fmt.Sprintf("sed -i.bak 's#%s #%s #g' %s", f.Bin, pathBin, file))
		}
	}
	switch {
	case len(seen) == 0:
	case len(bad) > 0:
		detail := "hooks run " + strings.Join(bad, ", ")
		if pathBin != "" {
			detail += "; PATH has " + pathBin
		} else {
			fixes = append(fixes, "put downshift on PATH (see the binary on PATH check)")
		}
		add("hook binary", doctorFail, detail, strings.Join(fixes, "\n"))
	case pathBin == "":
		add("hook binary", doctorWarn, "hooks run "+good+", but downshift is not on PATH",
			"ln -sf "+good+" "+filepath.Join(env.Home, ".local", "bin", "downshift"))
	default:
		add("hook binary", doctorOK, "hooks run "+good+" (same as PATH)", "")
	}
}
