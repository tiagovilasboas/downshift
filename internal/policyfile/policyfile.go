// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package policyfile reads an optional per-project tier floor. A floor only
// raises the tier a spawn is routed to; it never lowers one. A missing file is
// a no-op. An invalid file is reported to the caller, which must fail open
// (route as if there were no file) and may surface the error in diagnostics.
package policyfile

import (
	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/hookctx"
	"github.com/tiagovilasboas/downshift/internal/routing/policy"
)

func init() {
	core.TierFloorHook = func(d *core.Decision, currentModelID string, r core.Resolver) {
		// Only inside a hook invocation: no project directory, no floor.
		if dir := hookctx.ProjectDir(); dir != "" {
			ApplyFloor(d, dir, currentModelID, r)
		}
	}
}

const (
	// RelPath is the policy file location relative to a project directory.
	RelPath = policy.FloorRelPath
	// MaxDepth bounds the upward search from the starting directory.
	MaxDepth = policy.FloorMaxDepth
	maxBytes = policy.FloorMaxBytes
)

// Policy is the on-disk tier floor; the logic lives in internal/routing/policy.
type Policy = policy.FloorPolicy

// Find returns the nearest policy file at or above startDir.
func Find(startDir string) (string, bool, error) { return policy.FindFloor(startDir) }

// LoadNearest finds and parses the nearest policy.
func LoadNearest(startDir string) (*Policy, string, error) {
	return policy.LoadNearestFloor(startDir)
}

// Load reads and validates one policy file.
func Load(path string) (*Policy, error) { return policy.LoadFloor(path) }

// ParseTier maps a tier name to core.Tier.
func ParseTier(s string) (core.Tier, bool) { return policy.ParseFloorTier(s) }

// ApplyFloor raises d to the project's tier floor when a valid policy applies.
func ApplyFloor(d *core.Decision, cwd, currentModel string, res core.Resolver) {
	policy.ApplyFloor(d, cwd, currentModel, res)
}
