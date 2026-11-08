// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"path/filepath"
	"slices"

	"github.com/tiagovilasboas/downshift-ai/internal/console/setupwizard"
	"github.com/tiagovilasboas/downshift-ai/internal/setup"
)

// setupComponent is one wizard choice and the flag that expresses it. The
// table is the only place that maps a component to a setup flag, in both
// directions: Selection to options (set) and Selection to args (flags).
type setupComponent struct {
	id, label, hint string
	// defaultOn is the initial state, given what the command line already set.
	defaultOn func(o setup.Options) bool
	// set applies the choice to the options and to the switches setup resolves itself.
	set func(o *setup.Options, f *setupArgFlags, on bool)
	// offFlag is the argument that says "off" (--no-x); onFlag says "on" for
	// a component that is off unless asked for.
	offFlag, onFlag string
	destructive     bool
}

var setupComponents = []setupComponent{
	{id: "mcp", label: "MCP server", hint: "register the downshift MCP server in Claude Code, Codex and others",
		defaultOn: func(o setup.Options) bool { return o.Mcp },
		set:       func(o *setup.Options, _ *setupArgFlags, on bool) { o.Mcp = on }, offFlag: "--no-mcp"},
	{id: "plugin", label: "Claude Code plugin", hint: "the plugin carries the hooks instead of settings.json",
		defaultOn: func(o setup.Options) bool { return !o.NoPlugin },
		set:       func(o *setup.Options, _ *setupArgFlags, on bool) { o.NoPlugin = !on }, offFlag: "--no-plugin"},
	{id: "mods", label: "downshift-agents mod", hint: "live band and pane for subagents; it only observes",
		defaultOn: func(o setup.Options) bool { return !o.NoMods },
		set:       func(o *setup.Options, _ *setupArgFlags, on bool) { o.NoMods = !on }, offFlag: "--no-mods"},
	{id: "session-policy", label: "Session policy", hint: "Claude Code model opusplan and plan mode, only for keys not set",
		defaultOn: func(o setup.Options) bool { return !o.NoSessionPolicy },
		set:       func(o *setup.Options, _ *setupArgFlags, on bool) { o.NoSessionPolicy = !on }, offFlag: "--no-session-policy"},
	{id: "skills", label: "Skills", hint: "Downshift skills in the harness skills directory",
		defaultOn: func(o setup.Options) bool { return !o.NoSkills },
		set:       func(o *setup.Options, _ *setupArgFlags, on bool) { o.NoSkills = !on }, offFlag: "--no-skills"},
	{id: "steering", label: "Steering and agent triggers", hint: "the harness instructions file",
		defaultOn: func(o setup.Options) bool { return !o.NoSteering },
		set:       func(o *setup.Options, _ *setupArgFlags, on bool) { o.NoSteering = !on }, offFlag: "--no-steering"},
	{id: "policy", label: "Project policy offer", hint: "offer .downshift/policy.json in the current directory",
		defaultOn: func(o setup.Options) bool { return o.ProjectDir != "" },
		set:       func(_ *setup.Options, f *setupArgFlags, on bool) { f.noPolicy = !on }, offFlag: "--no-policy"},
	{id: "you-should-know", label: "You should know", hint: "Claude Code mod; each check is a billed background request",
		defaultOn: func(o setup.Options) bool { return o.YouShouldKnow },
		set:       func(o *setup.Options, _ *setupArgFlags, on bool) { o.YouShouldKnow = on }, onFlag: "--you-should-know"},
	{id: "prune", label: "Remove other downshift binaries", hint: "older copies on PATH and in the Go and per-user bin directories",
		defaultOn: func(setup.Options) bool { return false },
		set:       func(_ *setup.Options, f *setupArgFlags, on bool) { f.keepBinaries = !on }, offFlag: "--keep-binaries", destructive: true},
}

// wizardComponents lists the choices: hooks first, fixed, then the table.
func wizardComponents(o setup.Options) []setupwizard.Component {
	cs := []setupwizard.Component{{ID: "hooks", Label: "Hooks", Hint: "model routing and the shell compactor",
		Default: true, Disabled: true, DisabledReason: "always on"}}
	for _, c := range setupComponents {
		cs = append(cs, setupwizard.Component{ID: c.id, Label: c.label, Hint: c.hint,
			Default: c.defaultOn(o), Destructive: c.destructive})
	}
	return cs
}

// wizardHarnesses lists every harness setup supports. A detected one starts
// selected, unless harness names on the command line chose others.
func wizardHarnesses(o setup.Options) []setupwizard.Harness {
	hs := make([]setupwizard.Harness, 0, len(setup.Targets))
	for _, t := range setup.Targets {
		hs = append(hs, setupwizard.Harness{
			ID: t.Name, Label: t.Name,
			Detected:  setup.IsDir(filepath.Join(o.Home, filepath.FromSlash(t.ConfigDir))),
			Unchecked: len(o.Names) > 0 && !slices.Contains(o.Names, t.Name),
		})
	}
	return hs
}

// selectionOptions maps the wizard's choice onto the options and switches that
// the flags would have set. It touches no directory: the caller fills
// ProjectDir and BinDirs from the switches.
func selectionOptions(base setup.Options, sel setupwizard.Selection) (setup.Options, setupArgFlags) {
	o, f := base, setupArgFlags{}
	o.Names = slices.Clone(sel.Harnesses)
	for _, c := range setupComponents {
		c.set(&o, &f, slices.Contains(sel.Components, c.id))
	}
	if sel.Choice != "" {
		o.Workers = sel.Choice
	}
	return o, f
}

// selectionArgs is the argument list that reproduces the choice.
func selectionArgs(sel setupwizard.Selection) []string {
	args := slices.Clone(sel.Harnesses)
	for _, c := range setupComponents {
		on := slices.Contains(sel.Components, c.id)
		switch {
		case on && c.onFlag != "":
			args = append(args, c.onFlag)
		case !on && c.offFlag != "":
			args = append(args, c.offFlag)
		}
	}
	if sel.Choice != "" {
		args = append(args, "--workers", sel.Choice)
	}
	return args
}
