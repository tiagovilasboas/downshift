// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package command describes the Downshift commands the console can run. The
// console never knows what a command does: it gets this list from the binary
// and runs each entry as a child process of the same binary.
package command

import "strings"

// Spec is one runnable entry of the console's command list.
type Spec struct {
	// Args is the argv after "downshift", for example ["report", "--days=7"].
	Args []string
	// Group is the heading the entry is listed under.
	Group string
	// Summary is the one-line description shown next to the command.
	Summary string
	// Writes marks a command that changes files or state; the console asks
	// before running it and the default answer is no.
	Writes bool
	// Terminal marks a command that needs the real terminal (it prompts, reads
	// stdin, or keeps running); the console hands the terminal over to it.
	Terminal bool
	// Plain marks a command that prints a line and exits (version): typed in
	// a terminal it never opens the console. The console still lists it.
	Plain bool
}

// Name is the command line as typed after "downshift".
func (s Spec) Name() string { return strings.Join(s.Args, " ") }

// Filter returns the specs whose name or summary contains every word of query,
// case-insensitively, keeping the input order. An empty query keeps all.
func Filter(specs []Spec, query string) []Spec {
	words := strings.Fields(strings.ToLower(query))
	var out []Spec
	for _, s := range specs {
		hay := strings.ToLower(s.Name() + " " + s.Summary + " " + s.Group)
		if containsAll(hay, words) {
			out = append(out, s)
		}
	}
	return out
}

// WithExtra returns the spec's args plus the words typed after its name, so a
// query "report --days=3" run on the "report" entry becomes report --days=3.
func WithExtra(s Spec, query string) []string {
	args := append([]string(nil), s.Args...)
	typed := strings.Fields(query)
	if len(typed) <= len(s.Args) {
		return args
	}
	for i, a := range s.Args {
		if typed[i] != a {
			return args
		}
	}
	return append(args, typed[len(s.Args):]...)
}

func containsAll(hay string, words []string) bool {
	for _, w := range words {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}
