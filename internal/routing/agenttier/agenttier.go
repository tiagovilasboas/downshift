// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package agenttier answers "which tier does this squad agent declare?". The
// table is data: the `default_tier:` line in the frontmatter of each
// agents/*.md file, embedded in the binary. It is pure and holds no model id;
// the catalog maps a tier to a model per harness.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift
package agenttier

import (
	"bufio"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"github.com/tiagovilasboas/downshift/agents"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
)

var (
	once      sync.Once
	table     map[string]contract.Tier
	playbooks map[string]string
)

func load() {
	table = Declared(agents.FS)
	playbooks = declaredPlaybooks(agents.FS)
}

// Of returns the default tier of a squad agent type. prefix is what the
// harness puts before the agent name (core.HarnessCapabilities.SquadAgentPrefix,
// for example "downshift:"; empty for a bare name). Any other type, or a
// definition without a valid tier, is TierUnknown (today's behaviour).
func Of(agentType, prefix string) contract.Tier {
	name, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(agentType)), strings.ToLower(prefix))
	if !ok {
		return contract.TierUnknown
	}
	once.Do(load)
	return table[name]
}

// NameOf returns the lower-case name of a squad agent type, or "" when the type
// is not a squad agent that declares a tier. prefix is as for Of.
func NameOf(agentType, prefix string) string {
	name, ok := strings.CutPrefix(strings.ToLower(strings.TrimSpace(agentType)), strings.ToLower(prefix))
	if !ok || Of(agentType, prefix) == contract.TierUnknown {
		return ""
	}
	return name
}

// Entry is one squad agent and the tier its definition declares.
type Entry struct {
	Agent    string `json:"agent"`
	Tier     string `json:"tier"`
	Playbook string `json:"playbook,omitempty"`
}

// List returns every squad agent with its declared tier, sorted by name. It is
// what the MCP tools show an orchestrator whose harness sends no agent type.
func List() []Entry {
	once.Do(load)
	out := make([]Entry, 0, len(table))
	for name, tier := range table {
		out = append(out, Entry{Agent: name, Tier: string(tier), Playbook: playbooks[name]})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Agent < out[j].Agent })
	return out
}

// Declared returns the tier each agent file in fsys declares, by lower-case
// name. Files without a name or a valid tier are left out.
func Declared(fsys fs.FS) map[string]contract.Tier {
	out := map[string]contract.Tier{}
	files, _ := fs.Glob(fsys, "*.md")
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			continue
		}
		name, tier, _ := frontmatter(string(data))
		if name != "" && tier.Valid() {
			out[strings.ToLower(name)] = tier
		}
	}
	return out
}

// declaredPlaybooks returns the `playbook:` each agent file in fsys declares,
// by lower-case name. Agents without one are left out.
func declaredPlaybooks(fsys fs.FS) map[string]string {
	out := map[string]string{}
	files, _ := fs.Glob(fsys, "*.md")
	for _, f := range files {
		data, err := fs.ReadFile(fsys, f)
		if err != nil {
			continue
		}
		if name, _, pb := frontmatter(string(data)); name != "" && pb != "" {
			out[strings.ToLower(name)] = pb
		}
	}
	return out
}

// frontmatter reads `name:`, `default_tier:` and `playbook:` from the leading
// --- block.
func frontmatter(doc string) (string, contract.Tier, string) {
	sc := bufio.NewScanner(strings.NewReader(doc))
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return "", contract.TierUnknown, ""
	}
	var name, playbook string
	var tier contract.Tier
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch strings.TrimSpace(key) {
		case "name":
			name = val
		case "default_tier":
			tier = contract.Tier(val)
		case "playbook":
			playbook = val
		}
	}
	return name, tier, playbook
}
