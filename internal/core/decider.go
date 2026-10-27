// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package core

import "sync"

// Request is everything a Decider sees for one spawn: the task text, the
// calling harness, the subagent type when the harness sent one, the model the
// spawn would run on (may be empty) and the catalog resolver (may be nil).
type Request struct {
	Prompt         string
	Harness        string
	AgentType      string
	CurrentModelID string
	Resolver       Resolver
}

// Decider produces the routing Decision for one spawn. Adapters never call
// it directly: they call Route or RouteAgent, which ask the active Decider.
// Channel planning (PlanSession) stays downstream and is the same for every
// Decider.
type Decider interface {
	Decide(Request) Decision
}

// DeciderFunc adapts a plain function to Decider.
type DeciderFunc func(Request) Decision

// Decide calls f(req).
func (f DeciderFunc) Decide(req Request) Decision { return f(req) }

var (
	deciderMu sync.RWMutex
	// activeDecider is the Decider Route asks. nil means the default (v1).
	activeDecider Decider
)

// SetDecider installs d as the Decider every Route call asks and returns a
// function that restores the previous one. Like SetConfig, it is called at
// the edge (once per process, or by a test); a nil d restores the default.
func SetDecider(d Decider) (restore func()) {
	deciderMu.Lock()
	prev := activeDecider
	activeDecider = d
	deciderMu.Unlock()
	return func() {
		deciderMu.Lock()
		activeDecider = prev
		deciderMu.Unlock()
	}
}

// currentDecider returns the installed Decider, or the v1 default.
func currentDecider() Decider {
	deciderMu.RLock()
	d := activeDecider
	deciderMu.RUnlock()
	if d == nil {
		return v1Decider{}
	}
	return d
}

// Route is the main entry point: given a task prompt, a harness, the current
// model id (may be empty), and an optional Resolver, produce a routing Decision.
//
// Without a Resolver the decision carries tiers but no model ids, so the
// caller fails open. Hooks always pass the catalog; offline commands may not.
func Route(prompt, harness, currentModelID string, r ...Resolver) Decision {
	return RouteAgent(prompt, harness, "", currentModelID, r...)
}

// RouteAgent is Route with the spawn's subagent type, when the harness sent
// one. It asks the active Decider (see SetDecider).
func RouteAgent(prompt, harness, agentType, currentModelID string, r ...Resolver) Decision {
	var res Resolver
	if len(r) > 0 && r[0] != nil {
		res = r[0]
	}
	return currentDecider().Decide(Request{
		Prompt:         prompt,
		Harness:        harness,
		AgentType:      agentType,
		CurrentModelID: currentModelID,
		Resolver:       res,
	})
}
