// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

// Package harness2export is the machine contract for the local Harness² lab.
// Status words live in docs/reference/harness-matrix.md. A test fails when this
// map drifts from that table.
package harness2export

// Maturity mirrors the status column of the harness matrix. It is not a semver.
type Maturity string

const (
	MaturityProduction   Maturity = "production"
	MaturityFunctional   Maturity = "functional"
	MaturityBeta         Maturity = "beta"
	MaturityExperimental Maturity = "experimental"
	MaturityConfigOnly   Maturity = "config_only"
)

// Role is what the local lab may do with a harness.
type Role string

const (
	// RoleMutate is a Completed harness: the lab may propose shared-layer edits.
	RoleMutate Role = "mutate"
	// RoleObserve computes the same routing, and waits for a honored rewrite.
	RoleObserve Role = "observe"
	// RoleClassifierOnly has no hook. The lab may score try and reasoning only.
	RoleClassifierOnly Role = "classifier_only"
)

// Row is one harness in the lab matrix.
type Row struct {
	ID                        string   `json:"id"`
	HasHook                   bool     `json:"has_hook"`
	Maturity                  Maturity `json:"maturity"`
	Role                      Role     `json:"role"`
	CanRewriteModel           bool     `json:"can_rewrite_model"`
	RecursiveHarness2Eligible bool     `json:"recursive_harness2_eligible"`
	Notes                     string   `json:"notes,omitempty"`
}

// maturityByHarness must match the first status table in harness-matrix.md.
var maturityByHarness = map[string]struct {
	Maturity Maturity
	Notes    string
}{
	"claude-code": {MaturityProduction, "rewrite honored; usage and child end recorded"},
	"cursor":      {MaturityFunctional, "rewrite emitted; honor unconfirmed on some plans"},
	"codex":       {MaturityFunctional, "rewrite inferred from follow-up spawn"},
	"opencode":    {MaturityBeta, "adapter and conformance green; live spawn not observed"},
	"kirocrew":    {MaturityExperimental, "policy block/retry; no in-place rewrite"},
	"antigravity": {MaturityExperimental, "one observed child; build-specific"},
	"grok":        {MaturityConfigOnly, "reasoning_effort in config.toml; no hook"},
}

// HookPresent reports whether the binary exposes a hook for id.
type HookPresent func(id string) bool

// CapsSnapshot is the capability subset the lab records.
type CapsSnapshot struct {
	CanRewriteModel        bool `json:"can_rewrite_model"`
	CanApplyEffort         bool `json:"can_apply_effort"`
	SingleModelReasoning   bool `json:"single_model_reasoning"`
	CanRewriteShellCommand bool `json:"can_rewrite_shell_command"`
}

// Matrix builds the lab matrix. hookPresent comes from the CLI registry.
func Matrix(hookPresent HookPresent, capsFor func(string) (CapsSnapshot, bool)) []Row {
	order := []string{"claude-code", "cursor", "codex", "opencode", "kirocrew", "antigravity", "grok"}
	out := make([]Row, 0, len(order))
	for _, id := range order {
		meta, ok := maturityByHarness[id]
		if !ok {
			continue
		}
		hasHook := hookPresent(id)
		caps, capsOK := capsFor(id)
		role := RoleOf(meta.Maturity)
		out = append(out, Row{
			ID:                        id,
			HasHook:                   hasHook,
			Maturity:                  meta.Maturity,
			Role:                      role,
			CanRewriteModel:           capsOK && caps.CanRewriteModel,
			RecursiveHarness2Eligible: role == RoleMutate && hasHook,
			Notes:                     meta.Notes,
		})
	}
	return out
}

// RoleOf maps a matrix status to a lab role.
// Completed (production) is the only mutation role.
func RoleOf(m Maturity) Role {
	switch m {
	case MaturityProduction:
		return RoleMutate
	case MaturityConfigOnly:
		return RoleClassifierOnly
	default:
		return RoleObserve
	}
}

// MaturityOf returns the catalogued maturity for a matrix status word.
func MaturityOf(status string) (Maturity, bool) {
	switch status {
	case "Completed":
		return MaturityProduction, true
	case "Functional":
		return MaturityFunctional, true
	case "Beta":
		return MaturityBeta, true
	case "Experimental":
		return MaturityExperimental, true
	case "Config only":
		return MaturityConfigOnly, true
	default:
		return "", false
	}
}
