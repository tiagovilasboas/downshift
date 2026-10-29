// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/quota"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/decide"
	"github.com/tiagovilasboas/downshift/internal/routing/hint"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

// init installs the routing v2 decider at the edge, like SetConfig: every
// adapter reaches it through core.Route.
func init() { core.SetDecider(routingDecider{now: time.Now}) }

// routingDecider reads the inputs of one spawn (project policy, tier floor,
// quota snapshot, catalog) and asks routing/decide. It holds no policy of its
// own. Any error leaves the spawn unchanged: the hook fails open.
type routingDecider struct {
	now func() time.Time
}

// Decide returns the v2 decision, or the unchanged-spawn decision on any error.
// The gathering lives in decideSpawn (apply_core.go), shared with `downshift
// route` and the MCP apply tool.
func (r routingDecider) Decide(req core.Request) core.Decision {
	d, _ := decideSpawn(req, hookctx.ProjectDir(), r.now())
	return d
}

// loadProjectPolicy reads the routing policy and the tier floor from the same
// project file (nearest at or above dir) plus the user's file. No project
// directory means no project file; a missing file is an empty policy.
func loadProjectPolicy(dir string) (*policy.Policy, *policy.FloorPolicy, bool) {
	loc := policy.Locations{}
	var floor *policy.FloorPolicy
	if dir != "" {
		path, found, err := policy.FindFloor(dir)
		if err != nil {
			return nil, nil, false
		}
		if found {
			if floor, err = policy.LoadFloor(path); err != nil {
				return nil, nil, false
			}
			loc.Project = path
		}
	}
	user, err := policy.UserPath()
	if err != nil {
		return nil, nil, false
	}
	loc.User = user
	pol, err := policy.Load(loc)
	if err != nil {
		return nil, nil, false
	}
	return pol, floor, true
}

// quotaReadings turns the stored snapshot of the harness and of every profile
// harness into readings. A missing or unreadable snapshot is no reading, and
// headroom treats an expired reading as unknown, so both pass.
func quotaReadings(harness string, pol *policy.Policy) []contract.Reading {
	seen := map[string]bool{}
	var out []contract.Reading
	add := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		if s, _ := quota.Load(id); s != nil {
			out = append(out, quota.Readings(*s, pol.QuotaTTL)...)
		}
	}
	add(harness)
	for _, p := range pol.Profiles {
		add(string(p.Harness))
	}
	return out
}

func requestedTier(req core.Request, cat decide.Catalog) contract.Tier {
	if cat == nil || req.CurrentModelID == "" {
		return contract.TierUnknown
	}
	if m, ok := cat.LookupByID(req.Harness, req.CurrentModelID); ok {
		return contract.TierOf(m.Tier)
	}
	return contract.TierUnknown
}

func contractRequest(req core.Request, h contract.Hint) contract.Request {
	return contract.Request{
		Harness:        contract.HarnessID(req.Harness),
		RequestedModel: req.CurrentModelID,
		AgentType:      hint.ParseAgentType(req.AgentType),
		Hint:           h,
	}
}
