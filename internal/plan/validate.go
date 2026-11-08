// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/playbook"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/agenttier"
)

// agentInfo maps a shipped agent name to its declared tier and playbook.
func agentInfo() map[string]agenttier.Entry {
	out := map[string]agenttier.Entry{}
	for _, e := range agenttier.List() {
		out[e.Agent] = e
	}
	return out
}

func lowerTrim(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// validate collects every problem in in, not just the first.
func validate(in Input, playbooks []string) []string {
	var errs []string
	if len(in.Parts) == 0 {
		return []string{"plan has no parts"}
	}
	agents := agentInfo()
	known := map[string]bool{}
	for _, p := range playbooks {
		known[p] = true
	}
	ids := map[string]bool{}
	for i, p := range in.Parts {
		label := fmt.Sprintf("part %d", i+1)
		switch {
		case strings.TrimSpace(p.ID) == "":
			errs = append(errs, label+": id is empty")
		case ids[p.ID]:
			errs = append(errs, fmt.Sprintf("%s: duplicate id %q", label, p.ID))
		default:
			ids[p.ID] = true
			label = fmt.Sprintf("part %q", p.ID)
		}
		errs = append(errs, validatePart(label, p, agents, known)...)
	}
	for i, p := range in.Parts {
		for _, d := range p.DependsOn {
			if !ids[d] {
				errs = append(errs, fmt.Sprintf("part %d (%s): depends_on %q does not exist", i+1, p.ID, d))
			}
		}
	}
	if len(errs) == 0 {
		errs = append(errs, cycles(in.Parts)...)
	}
	return errs
}

func validatePart(label string, p Part, agents map[string]agenttier.Entry, known map[string]bool) []string {
	var errs []string
	if _, ok := agents[lowerTrim(p.Agent)]; !ok {
		errs = append(errs, fmt.Sprintf("%s: unknown agent %q (known: %s)", label, p.Agent, strings.Join(agentNames(agents), ", ")))
	}
	switch p.Points {
	case 1, 2, 3, 5:
	case 8, 13:
		errs = append(errs, fmt.Sprintf("%s: %d points is over the 5 point ceiling: split this part", label, p.Points))
	default:
		errs = append(errs, fmt.Sprintf("%s: points %d is not one of 1, 2, 3, 5", label, p.Points))
	}
	switch p.Worker {
	case "", WorkerNative, WorkerClaude, WorkerCodex:
	default:
		errs = append(errs, fmt.Sprintf("%s: unknown worker %q (native, claude or codex)", label, p.Worker))
	}
	if p.Playbook != "" && !known[playbook.Slug(p.Playbook)] {
		errs = append(errs, fmt.Sprintf("%s: playbook %q does not exist", label, p.Playbook))
	}
	return errs
}

func agentNames(agents map[string]agenttier.Entry) []string {
	names := make([]string, 0, len(agents))
	for n := range agents {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// cycles reports a dependency cycle, if any. Ids and references are valid.
func cycles(parts []Part) []string {
	deps := map[string][]string{}
	for _, p := range parts {
		deps[p.ID] = p.DependsOn
	}
	state := map[string]int{} // 1 visiting, 2 done
	var visit func(id string) bool
	visit = func(id string) bool {
		switch state[id] {
		case 1:
			return true
		case 2:
			return false
		}
		state[id] = 1
		for _, d := range deps[id] {
			if visit(d) {
				return true
			}
		}
		state[id] = 2
		return false
	}
	for _, p := range parts {
		if visit(p.ID) {
			return []string{fmt.Sprintf("dependency cycle involving %q", p.ID)}
		}
	}
	return nil
}
