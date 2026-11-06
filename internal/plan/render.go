// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Markdown renders the plan: heading, table, totals and warnings.
func (p Plan) Markdown() string {
	var b strings.Builder
	goal := p.Goal
	if goal == "" {
		goal = "(no goal given)"
	}
	fmt.Fprintf(&b, "## Plan: %s\n\n", cell(goal))
	b.WriteString("| # | Part | Agent | Tier | Playbook | Pts | Depends on | Wave | Route |\n")
	b.WriteString("|---|------|-------|------|----------|-----|------------|------|-------|\n")
	for _, q := range p.Parts {
		deps := strings.Join(q.DependsOn, ", ")
		if deps == "" {
			deps = "-"
		}
		title := q.Title
		if title == "" {
			title = q.ID
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d | %s | %d | %s |\n",
			cell(q.ID), cell(title), q.Agent, q.Tier, q.Playbook, q.Points, cell(deps), q.Wave, q.Route)
	}
	fmt.Fprintf(&b, "\nTotal: %d points in %d waves.\n", p.Points, p.Waves)
	if len(p.Warnings) > 0 {
		b.WriteString("\nWarnings:\n")
		for _, w := range p.Warnings {
			fmt.Fprintf(&b, "- %s\n", w)
		}
	}
	b.WriteString(nextMarkdown(p.Next))
	return b.String()
}

// JSON renders the filled plan as indented JSON.
func (p Plan) JSON() ([]byte, error) {
	return json.MarshalIndent(p, "", "  ")
}

// cell keeps a value from breaking a markdown table row.
func cell(s string) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "|", "\\|")
	return strings.ReplaceAll(s, "\n", " ")
}
