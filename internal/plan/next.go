// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Kinds of next step.
const (
	KindSpawn    = "spawn"
	KindDelegate = "delegate"
)

// TierSmall is the tier whose hint a spawn prompt carries.
const TierSmall = "small"

// Step is one ready-to-run action: a native subagent spawn or the arguments
// of the MCP delegate tool.
type Step struct {
	Wave         int    `json:"wave"`
	Part         string `json:"part"`
	Kind         string `json:"kind"`
	SubagentType string `json:"subagent_type,omitempty"`
	Agent        string `json:"agent,omitempty"`
	Tier         string `json:"tier"`
	Playbook     string `json:"playbook"`
	Route        string `json:"route"`
	Mode         string `json:"mode,omitempty"`
	Prompt       string `json:"prompt"`
}

// nextSteps turns the filled parts into steps ordered by wave, then plan order.
func nextSteps(parts []FilledPart) []Step {
	steps := make([]Step, 0, len(parts))
	for _, q := range parts {
		s := Step{Wave: q.Wave, Part: q.ID, Tier: q.Tier, Playbook: q.Playbook, Route: q.Route, Prompt: promptOf(q)}
		if q.Worker == WorkerNative {
			s.Kind, s.SubagentType = KindSpawn, "downshift:"+q.Agent
		} else {
			s.Kind, s.Agent, s.Mode = KindDelegate, q.Worker, q.Mode
		}
		steps = append(steps, s)
	}
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].Wave < steps[j].Wave })
	return steps
}

// promptOf builds the task text. The small hint only moves a spawn down, and
// it is the first thing in the prompt.
func promptOf(q FilledPart) string {
	var b strings.Builder
	if q.Worker == WorkerNative && q.Tier == TierSmall {
		b.WriteString("[tier=small] ")
	}
	title := q.Title
	if title == "" {
		title = q.ID
	}
	fmt.Fprintf(&b, "Follow the %s playbook. Task: %s.", q.Playbook, strings.TrimSpace(title))
	if len(q.Files) > 0 {
		fmt.Fprintf(&b, " Files: %s.", strings.Join(q.Files, ", "))
	}
	if len(q.DependsOn) > 0 {
		fmt.Fprintf(&b, " Depends on: %s (already done).", strings.Join(q.DependsOn, ", "))
	}
	return b.String()
}

// delegateArgs is the MCP delegate tool input for a step. The tool takes no
// tier or playbook: the playbook is named inside the task text.
func delegateArgs(s Step) string {
	b, _ := json.Marshal(map[string]any{"agent": s.Agent, "mode": s.Mode, "tasks": []string{s.Prompt}})
	return string(b)
}

// nextMarkdown renders the "Next steps" section, one block per wave.
func nextMarkdown(steps []Step) string {
	if len(steps) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Next steps\n")
	wave, usesWorker := 0, false
	for _, s := range steps {
		if s.Wave != wave {
			wave = s.Wave
			fmt.Fprintf(&b, "\nWave %d (run its parts in parallel):\n", wave)
		}
		if s.Kind == KindSpawn {
			fmt.Fprintf(&b, "- %s: Agent tool, subagent_type `%s`, tier %s, prompt: %s\n", s.Part, s.SubagentType, s.Tier, s.Prompt)
			continue
		}
		usesWorker = true
		fmt.Fprintf(&b, "- %s: MCP `delegate` %s (tier %s, playbook %s, route %s)\n", s.Part, delegateArgs(s), s.Tier, s.Playbook, s.Route)
	}
	if usesWorker {
		b.WriteString("\nDelegate steps route through the gateway only when it is on. Read results with `collect`.\n")
	}
	return b.String()
}
