// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"fmt"
	"io"
	"sort"
)

// AgentTypeCount is how many decisions a subagent type had, and how many of
// those did not know the model the child would have run on.
type AgentTypeCount struct {
	Decisions       int `json:"decisions"`
	UnknownBaseline int `json:"unknown_baseline"`
}

// builtinAgentTypes are the subagent types Claude Code ships. They are safe to
// record by name: they say nothing about the user's work.
var builtinAgentTypes = map[string]bool{
	"general-purpose":   true,
	"claude":            true,
	"Explore":           true,
	"Plan":              true,
	"statusline-setup":  true,
	"claude-code-guide": true,
}

// AgentTypeLabel turns a subagent type into what may be stored and exported.
// stats --export is pasted into public issues, and a custom agent's name (a
// reviewer for one product, say) can say something about the user's work, so
// only the built-in names are kept. A call with no type is "unspecified";
// everything else is "custom". Names are case-sensitive, as in Claude Code.
func AgentTypeLabel(raw string) string {
	switch {
	case raw == "":
		return "unspecified"
	case builtinAgentTypes[raw]:
		return raw
	default:
		return "custom"
	}
}

// printAgentTypes lists decisions per subagent type with how many had no known
// baseline, the type with the most unknown first. Nothing is printed when no
// event carries a type (logs written before the field existed).
func printAgentTypes(w io.Writer, byType map[string]AgentTypeCount) {
	if len(byType) == 0 {
		return
	}
	names := make([]string, 0, len(byType))
	for name := range byType {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := byType[names[i]], byType[names[j]]
		if a.UnknownBaseline != b.UnknownBaseline {
			return a.UnknownBaseline > b.UnknownBaseline
		}
		if a.Decisions != b.Decisions {
			return a.Decisions > b.Decisions
		}
		return names[i] < names[j]
	})
	fmt.Fprintf(w, "\nBaseline by agent type\n")
	for _, name := range names {
		c := byType[name]
		fmt.Fprintf(w, "  %-20s %5d decisions  %5d unknown  (%.0f%%)\n", name, c.Decisions, c.UnknownBaseline, pct(c.UnknownBaseline, c.Decisions))
	}
}
