// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"runtime"
	"sort"

	"github.com/tiagovilasboas/downshift-ai/internal/core"
	"github.com/tiagovilasboas/downshift-ai/internal/routing/policy"
	"github.com/tiagovilasboas/downshift-ai/internal/workers"
)

// workerAgentLists reads the worker agents from the harness capabilities:
// every agent a harness can run as a worker, and those whose read-only recipe
// has an observed refused write (the only ones the default turns on).
func workerAgentLists() (known, verified []string) {
	for _, n := range core.AllHarnessCaps() {
		if n.Caps.WorkerAgent == "" {
			continue
		}
		known = append(known, n.Caps.WorkerAgent)
		if n.Caps.WorkerReadOnlyVerified {
			verified = append(verified, n.Caps.WorkerAgent)
		}
	}
	sort.Strings(known)
	sort.Strings(verified)
	return known, verified
}

// policyLocations returns the project policy file under root (when root is
// set) and the user's policy file.
func policyLocations(root string) policy.Locations {
	loc := policy.Locations{}
	if root != "" {
		loc.Project = policy.ProjectPath(root)
	}
	if u, err := policy.UserPath(); err == nil {
		loc.User = u
	}
	return loc
}

// resolveWorkerMode is the one place the worker mode is computed, for the MCP
// server, doctor, setup and plan alike: env, then the user file, then the
// project file (which may only lower it), then the default.
func resolveWorkerMode(root string, getenv func(string) string) policy.Workers {
	known, verified := workerAgentLists()
	return policy.ResolveWorkers(getenv, policyLocations(root), known, verified, runtime.GOOS)
}

// workerLaunchers builds the worker routes for a resolved mode: none when off,
// the read-only route for the enabled agents when read, and the write route
// too when write. A write worker runs in its own git worktree (spec section 7).
func workerLaunchers(w policy.Workers) workers.Launcher {
	switch w.Mode {
	case policy.WorkersRead:
		return workers.Routes(workers.ByAgent(onlyAgents(workers.ReadOnlyLaunchers(), w.Agents)), nil)
	case policy.WorkersWrite:
		return workers.Routes(
			workers.ByAgent(onlyAgents(workers.ReadOnlyLaunchers(), w.Agents)),
			workers.ByAgent(onlyAgents(workers.WriteLaunchers(), w.Agents)))
	}
	return workers.Routes(workers.Unverified(), nil)
}

// onlyAgents keeps the launchers of the enabled agents.
func onlyAgents(all map[string]workers.Launcher, agents []string) map[string]workers.Launcher {
	out := map[string]workers.Launcher{}
	for _, a := range agents {
		if l, ok := all[a]; ok {
			out[a] = l
		}
	}
	return out
}
