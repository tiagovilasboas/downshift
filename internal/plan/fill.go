// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package plan

import (
	"sort"
	"strconv"
	"strings"

	"github.com/tiagovilasboas/downshift-ai/internal/playbook"
)

// fill applies the defaults: tier and playbook from the agent, route from the
// worker and the gateway. The part is already validated. A worker part the
// worker state cannot run becomes native, with a warning saying why.
func fill(p Part, gw Gateway, w Workers) (FilledPart, string) {
	info := agentInfo()[lowerTrim(p.Agent)]
	p.Agent = lowerTrim(p.Agent)
	if p.Worker == "" {
		p.Worker = WorkerNative
	}
	if p.Playbook == "" {
		p.Playbook = info.Playbook
	} else {
		p.Playbook = playbook.Slug(p.Playbook)
	}
	mode, warn := "", ""
	if p.Worker != WorkerNative {
		if warn = workerWarning(p, w); warn != "" {
			p.Worker = WorkerNative
		} else if mode = WorkersRead; editsFiles(p.Playbook) {
			mode = WorkersWrite
		}
	}
	return FilledPart{Part: p, Tier: info.Tier, Route: routeOf(p.Worker, gw), Mode: mode}, warn
}

// routeOf is "native" for the harness's own subagents; claude and codex
// workers go through the gateway when it is on, else straight to the vendor.
func routeOf(worker string, gw Gateway) string {
	if worker == WorkerNative {
		return WorkerNative
	}
	if gw.Enabled && gw.Name != "" {
		return gw.Name
	}
	return RouteDirect
}

// waveOf is the 1-based topological level of each part: 1 + the deepest
// dependency. The graph is acyclic.
func waveOf(parts []Part) map[string]int {
	deps := map[string][]string{}
	for _, p := range parts {
		deps[p.ID] = p.DependsOn
	}
	wave := map[string]int{}
	var level func(id string) int
	level = func(id string) int {
		if w, ok := wave[id]; ok {
			return w
		}
		w := 1
		for _, d := range deps[id] {
			if l := level(d) + 1; l > w {
				w = l
			}
		}
		wave[id] = w
		return w
	}
	for _, p := range parts {
		level(p.ID)
	}
	return wave
}

// sharedFiles warns about parts of one wave that name the same file, since
// they run in parallel.
func sharedFiles(parts []FilledPart) []string {
	type key struct {
		wave int
		file string
	}
	owners := map[key][]string{}
	for _, p := range parts {
		seen := map[string]bool{}
		for _, f := range p.Files {
			f = strings.TrimSpace(f)
			if f == "" || seen[f] {
				continue
			}
			seen[f] = true
			k := key{p.Wave, f}
			owners[k] = append(owners[k], p.ID)
		}
	}
	var out []string
	for k, ids := range owners {
		if len(ids) > 1 {
			out = append(out, "wave "+strconv.Itoa(k.wave)+": "+strings.Join(ids, ", ")+" share file "+k.file)
		}
	}
	sort.Strings(out)
	return out
}
