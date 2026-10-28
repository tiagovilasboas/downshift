// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/routing/contract"
	"github.com/tiagovilasboas/downshift/internal/routing/decide"
	"github.com/tiagovilasboas/downshift/internal/routing/hint"
)

// decideSpawn is the one answer to "what would Downshift apply for this
// spawn": the hook (routingDecider), `downshift route` and the MCP apply tool
// all call it. It gathers the project policy, the tier floor, the quota
// snapshot and the catalog for dir (empty means no project file), asks
// routing/decide, and returns the decision with its routing note. Any error
// leaves the spawn unchanged: doubt keeps.
func decideSpawn(req core.Request, dir string, now time.Time) (core.Decision, contract.Note) {
	d, n, _ := decideSpawnStatus(req, dir, now)
	return d, n
}

// decideSpawnStatus is decideSpawn plus whether the project policy could be
// used. A policy or floor that cannot be read or validated keeps the requested
// model, and the caller can say so.
func decideSpawnStatus(req core.Request, dir string, now time.Time) (core.Decision, contract.Note, bool) {
	cat, _ := req.Resolver.(decide.Catalog)
	in, ok := gatherInput(req, cat, dir, now)
	if !ok {
		in = unchangedInput(req, cat)
	}
	d, n := decide.Decide(in)
	return d, n, ok
}

// gatherInput reads everything the decision needs. It reports false on a
// policy or floor that cannot be used.
func gatherInput(req core.Request, cat decide.Catalog, dir string, now time.Time) (decide.Input, bool) {
	pol, floor, ok := loadProjectPolicy(dir)
	if !ok {
		return decide.Input{}, false
	}
	h := hint.Parse(req.Prompt, requestedTier(req, cat))
	return decide.Input{
		Request:  contractRequest(req, h),
		Policy:   pol,
		Readings: quotaReadings(req.Harness, pol),
		Now:      now,
		Risk:     core.RiskCategories(req.Prompt),
		Catalog:  cat,
		Floor:    floor,
		Class:    core.Classify(req.Prompt).Complexity.String(),
	}, true
}

// unchangedInput is a spawn with no policy, hint, floor or risk: v2 keeps the
// requested model for it. The agent type is dropped too: no risk scan ran, so
// the default profile table must not move the spawn.
func unchangedInput(req core.Request, cat decide.Catalog) decide.Input {
	r := contractRequest(req, contract.Hint{})
	r.AgentType = ""
	return decide.Input{Request: r, Catalog: cat}
}

// ApplyInput is one spawn the orchestrator asks about. It carries ids and
// enums only: no task text.
type ApplyInput struct {
	Harness        string
	RequestedModel string // the orchestrator's explicit choice; may be empty
	Effort         string // requested reasoning effort; echoed when nothing moves
	AgentType      string
	TierHint       string // "", "small", "mid" or "frontier"
	ProjectDir     string // empty means no project policy file
}

// ApplyResult is what Downshift would apply. Note holds enums, tiers and
// numbers only.
type ApplyResult struct {
	Harness        string
	RequestedModel string
	SelectedModel  string
	Tier           string // tier of the selected model; empty when unknown
	Effort         string
	Verdict        string
	Reason         string
	Applied        bool // true when the decision changes the requested model
	Note           map[string]any
}

// Applier answers what Downshift would apply for a spawn. The MCP apply tool
// receives it; it imports nothing from the MCP packages.
type Applier interface {
	Apply(ctx context.Context, in ApplyInput) (ApplyResult, error)
}

// spawnApplier wires the catalog, the policy and the quota into decideSpawn.
type spawnApplier struct {
	cat core.Resolver
	now func() time.Time
}

// Apply decides the spawn through decideSpawn. A tier hint is passed in the
// same `[tier=...]` form the hook reads, so both paths share one parser.
func (a spawnApplier) Apply(ctx context.Context, in ApplyInput) (ApplyResult, error) {
	if err := ctx.Err(); err != nil {
		return ApplyResult{}, err
	}
	if err := validateApplyInput(in); err != nil {
		return ApplyResult{}, err
	}
	req := core.Request{
		Harness: in.Harness, CurrentModelID: in.RequestedModel,
		AgentType: in.AgentType, Resolver: a.cat,
	}
	if contract.Tier(in.TierHint).Valid() {
		req.Prompt = "[tier=" + in.TierHint + "]"
	}
	d, note, policyOK := decideSpawnStatus(req, in.ProjectDir, a.now())
	return applyResult(in, d, note, a.knows(in), policyOK), nil
}

// knows reports whether the catalog lists the requested model for the harness.
func (a spawnApplier) knows(in ApplyInput) bool {
	cat, ok := a.cat.(decide.Catalog)
	if !ok || in.RequestedModel == "" {
		return false
	}
	_, found := cat.LookupByID(in.Harness, in.RequestedModel)
	return found
}

func applyResult(in ApplyInput, d core.Decision, note contract.Note, known, policyOK bool) ApplyResult {
	res := ApplyResult{
		Harness: in.Harness, RequestedModel: in.RequestedModel,
		SelectedModel: in.RequestedModel, Effort: in.Effort,
		Tier: string(note.TierTo), Verdict: string(note.Verdict),
		Note: noteMap(note),
	}
	moved := (d.Verdict == core.VerdictDownshift || d.Verdict == core.VerdictUpshift) &&
		d.Model.ID != "" && d.Model.ID != in.RequestedModel
	if moved {
		res.SelectedModel, res.Effort, res.Applied = d.Model.ID, d.Effort.String(), true
	}
	res.Reason = applyReason(note, moved, known, in.RequestedModel != "", policyOK)
	return res
}

// applyReason is one sentence built from enums and declared ids only.
func applyReason(n contract.Note, moved, known, named, policyOK bool) string {
	switch {
	case moved && n.RuleID != "":
		return "declared rule " + string(n.RuleID) + " applied to the requested model"
	case moved && n.Profile != "":
		return "declared profile " + string(n.Profile) + " applied to the requested model"
	case moved && n.Verdict == contract.VerdictHintDown:
		return "tier hint applied: moved down from the requested model"
	case moved:
		return "declared policy applied (" + string(n.Verdict) + ")"
	case !named:
		return "no requested model: nothing to apply"
	case !policyOK:
		return "the project policy could not be read or is invalid: the requested model is kept (doubt keeps)"
	case !known:
		return "requested model is not in the catalog for this harness: kept (doubt keeps)"
	case n.Reason != "":
		return "requested model kept: " + string(n.Reason)
	default:
		return "requested model kept: no declared policy or hint moved it"
	}
}

// noteMap turns the note into a map through its own JSON tags, so a field
// added to the note reaches the surface without a second list to maintain.
func noteMap(n contract.Note) map[string]any {
	raw, err := json.Marshal(n)
	m := map[string]any{}
	if err == nil {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

// effortNames are the reasoning-effort names the catalog uses.
var effortNames = []string{"low", "medium", "high", "xhigh", "max"}

// ApplyInputError is a refusal of the input itself. Its message is a fixed
// sentence that never repeats the value it refuses.
type ApplyInputError struct{ Msg string }

func (e *ApplyInputError) Error() string { return e.Msg }

// validateApplyInput refuses a tier hint or effort that names no known value,
// the same for the command and the MCP tool.
func validateApplyInput(in ApplyInput) error {
	if in.TierHint != "" && !contract.Tier(in.TierHint).Valid() {
		return &ApplyInputError{Msg: "tier must be one of: small, mid, frontier"}
	}
	if in.Effort != "" && !slices.Contains(effortNames, in.Effort) {
		return &ApplyInputError{Msg: "effort must be one of: " + strings.Join(effortNames, ", ")}
	}
	return nil
}
