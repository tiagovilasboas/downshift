// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package clispec is the root command table of the downshift binary as data:
// one entry per dispatched command, with its summary, flags and visibility.
// The help, usage and typo suggestions in cmd/downshift read it, and so do
// the documentation guards in internal/docscheck and the docs-manifest
// command. Dispatch does not read it: main.go's switch stays the behaviour,
// and cmd/downshift tests prove the two agree.
package clispec

import "sort"

// Flag is one option of a command.
type Flag struct {
	Name       string `json:"name"`                 // long form with dashes, for example "--days"
	Short      string `json:"short,omitempty"`      // short form, for example "-y"
	Deprecated bool   `json:"deprecated,omitempty"` // still accepted, no longer documented as current
}

// Command is one dispatched command, or one subcommand that has its own help
// (named "models list").
type Command struct {
	Name    string
	Summary string
	Flags   []Flag
	Hidden  bool   // callable, but not offered in help, typo suggestions or docs
	AliasOf string // the canonical command when Name is a legacy or alias name
	Legacy  bool   // kept for old hook or shell rc files; not suggested for a typo
}

// Commands returns a copy of the table, sorted by name.
func Commands() []Command {
	out := make([]Command, len(table))
	copy(out, table)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Lookup returns the entry for name.
func Lookup(name string) (Command, bool) {
	for _, c := range table {
		if c.Name == name {
			return c, true
		}
	}
	return Command{}, false
}

// LegacyNames maps each legacy dispatch name to the command it replaced ("" when
// it replaced none).
func LegacyNames() map[string]string {
	m := map[string]string{}
	for _, c := range table {
		if c.Legacy {
			m[c.Name] = c.AliasOf
		}
	}
	return m
}

// Suggestable lists the top-level commands a typo can be corrected to, sorted.
func Suggestable() []string {
	var names []string
	for _, c := range table {
		if c.Hidden || c.Legacy || c.AliasOf != "" || containsSpace(c.Name) {
			continue
		}
		names = append(names, c.Name)
	}
	sort.Strings(names)
	return names
}

func containsSpace(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return true
		}
	}
	return false
}

// f builds a flag list from long names; a "name|-s" form carries the short
// form, and a leading "!" marks the flag deprecated.
func f(specs ...string) []Flag {
	out := make([]Flag, 0, len(specs))
	for _, s := range specs {
		fl := Flag{}
		if s[0] == '!' {
			fl.Deprecated, s = true, s[1:]
		}
		for i := 0; i < len(s); i++ {
			if s[i] == '|' {
				fl.Short, s = s[i+1:], s[:i]
				break
			}
		}
		fl.Name = s
		out = append(out, fl)
	}
	return out
}

var table = concat(dailyCommands, modelCommands, plumbingCommands)

func concat(groups ...[]Command) []Command {
	var all []Command
	for _, g := range groups {
		all = append(all, g...)
	}
	return all
}
