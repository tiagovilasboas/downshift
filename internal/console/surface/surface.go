// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package surface decides, from facts alone, whether an invocation opens the
// full-screen console or stays plain text. It is pure: the caller collects the
// terminal and environment facts. Doubt means plain.
package surface

import (
	"slices"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/console/shared/command"
	"github.com/tiagovilasboas/downshift-ai/internal/core"
)

// Facts is everything the decision reads.
type Facts struct {
	// Args is the argv after "downshift"; empty means a bare invocation.
	Args []string
	// StdinTTY and StdoutTTY say whether each stream is a terminal.
	StdinTTY, StdoutTTY bool
	// Getenv reads an environment variable; nil reads as always empty.
	Getenv func(string) string
}

// Decision is the outcome and a short stable id for why.
type Decision struct {
	Console bool
	Reason  string
}

// machineFlags ask for output meant for a program, never for the console.
var machineFlags = []string{"--json", "--brief", "--plain", "--panel", "--export", "--quiet", "-o"}

// Decide applies the ordered rules; the first match wins.
func Decide(f Facts, specs []command.Spec) Decision {
	env := f.Getenv
	if env == nil {
		env = func(string) string { return "" }
	}
	if env("DOWNSHIFT_PLAIN") != "" {
		return plain("env:DOWNSHIFT_PLAIN")
	}
	if flag := machineFlag(f.Args); flag != "" {
		return plain("flag:" + flag)
	}
	if len(f.Args) > 0 && !isReadOnlySpec(f.Args, specs) {
		return plain("command")
	}
	if !f.StdinTTY || !f.StdoutTTY {
		return plain("no-tty")
	}
	if env("CI") != "" {
		return plain("ci")
	}
	if env("TERM") == "dumb" {
		return plain("term-dumb")
	}
	if name := agentVar(env); name != "" {
		return plain("agent-env:" + name)
	}
	return Decision{Console: true, Reason: "console"}
}

func plain(reason string) Decision { return Decision{Reason: reason} }

func machineFlag(args []string) string {
	for _, a := range args {
		for _, m := range machineFlags {
			if a == m || strings.HasPrefix(a, m+"=") {
				return m
			}
		}
	}
	return ""
}

// isReadOnlySpec reports whether args are exactly one spec that neither writes
// nor needs the real terminal.
func isReadOnlySpec(args []string, specs []command.Spec) bool {
	for _, s := range specs {
		if slices.Equal(s.Args, args) {
			return !s.Writes && !s.Terminal && !s.Plain
		}
	}
	return false
}

// agentVar returns the first set variable any harness declares for its shell.
func agentVar(env func(string) string) string {
	for _, n := range core.AllHarnessCaps() {
		for _, v := range agentVars(n.Caps) {
			if env(v) != "" {
				return v
			}
		}
	}
	return ""
}

// agentVars is the union of a harness's declared shell variables.
func agentVars(c core.HarnessCapabilities) []string {
	var vars []string
	for _, v := range append([]string{c.ShellEnv, c.SessionEnv}, c.AgentEnv...) {
		if v != "" {
			vars = append(vars, v)
		}
	}
	return vars
}
