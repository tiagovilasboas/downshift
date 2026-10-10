// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/outcome"
)

// OutcomeEvalReport compares classifier baseline routing to adapt-adjusted tiers
// using recorded pass/fail outcomes per tier (no model calls).
type OutcomeEvalReport struct {
	Total        int
	BaselinePass int
	AdjustedPass int
	Down         int
	Up           int
	Same         int
}

func (r OutcomeEvalReport) BaselineRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.BaselinePass) / float64(r.Total)
}

func (r OutcomeEvalReport) AdjustedRate() float64 {
	if r.Total == 0 {
		return 0
	}
	return float64(r.AdjustedPass) / float64(r.Total)
}

// EvalContrafactualPerTask builds tier memory from each task's own small/mid/frontier
// runs, then applies AdjustTier. This is an upper bound on what perfect per-task
// memory could achieve, not what shared adapt-memory.json does in production.
func EvalContrafactualPerTask(tasks []outcome.Task, runs []outcome.Run) (OutcomeEvalReport, error) {
	byTier, err := indexRuns(runs)
	if err != nil {
		return OutcomeEvalReport{}, err
	}
	var rep OutcomeEvalReport
	for _, t := range tasks {
		key := memoryKeyForPrompt(t.Prompt)
		if key == "" {
			continue
		}
		mem := memoryFromTaskOutcomes(t.ID, t.Prompt, byTier)
		baseline := core.ClassifyWithSemantic(t.Prompt).Complexity.Tier()
		adjusted := Tier(baseline)
		if mem != nil {
			adjusted = AdjustTier(Tier(baseline), key, mem)
		}
		bPass, okB := passAt(baseline, t.ID, byTier)
		aPass, okA := passAt(core.Tier(adjusted), t.ID, byTier)
		if !okB || !okA {
			return OutcomeEvalReport{}, fmt.Errorf("task %s: missing tier run data", t.ID)
		}
		rep.Total++
		if bPass {
			rep.BaselinePass++
		}
		if aPass {
			rep.AdjustedPass++
		}
		switch {
		case adjusted < Tier(baseline):
			rep.Down++
		case adjusted > Tier(baseline):
			rep.Up++
		default:
			rep.Same++
		}
	}
	return rep, nil
}

// EvalSharedShapeMemory aggregates hits/misses across all tasks sharing a dominant
// shape, then applies AdjustTier per task — what adapt-memory.json does when shapes
// collide across unrelated prompts.
func EvalSharedShapeMemory(tasks []outcome.Task, runs []outcome.Run) (OutcomeEvalReport, error) {
	byTier, err := indexRuns(runs)
	if err != nil {
		return OutcomeEvalReport{}, err
	}
	var events []FeedbackEvent
	for _, t := range tasks {
		key := memoryKeyForPrompt(t.Prompt)
		if key == "" {
			continue
		}
		events = append(events, feedbackFromTask(t.ID, key, byTier)...)
	}
	global := BuildMemory(events)
	var rep OutcomeEvalReport
	for _, t := range tasks {
		key := memoryKeyForPrompt(t.Prompt)
		if key == "" {
			continue
		}
		baseline := core.ClassifyWithSemantic(t.Prompt).Complexity.Tier()
		adjusted := AdjustTier(Tier(baseline), key, &global)
		bPass, okB := passAt(baseline, t.ID, byTier)
		aPass, okA := passAt(core.Tier(adjusted), t.ID, byTier)
		if !okB || !okA {
			return OutcomeEvalReport{}, fmt.Errorf("task %s: missing tier run data", t.ID)
		}
		rep.Total++
		if bPass {
			rep.BaselinePass++
		}
		if aPass {
			rep.AdjustedPass++
		}
		switch {
		case adjusted < Tier(baseline):
			rep.Down++
		case adjusted > Tier(baseline):
			rep.Up++
		default:
			rep.Same++
		}
	}
	return rep, nil
}

func memoryKeyForPrompt(prompt string) string {
	return MemoryKeyForPrompt(prompt, "")
}

type tierResults map[core.Tier]map[string]bool

func indexRuns(runs []outcome.Run) (tierResults, error) {
	out := tierResults{
		core.TierSmall:    {},
		core.TierMid:      {},
		core.TierFrontier: {},
	}
	for _, r := range runs {
		t, ok := runTier(r.Tier)
		if !ok {
			return nil, fmt.Errorf("unknown run tier %q", r.Tier)
		}
		for id, pass := range r.Results {
			out[t][id] = pass
		}
	}
	return out, nil
}

func runTier(label string) (core.Tier, bool) {
	switch label {
	case "small":
		return core.TierSmall, true
	case "mid":
		return core.TierMid, true
	case "frontier":
		return core.TierFrontier, true
	default:
		return 0, false
	}
}

func passAt(tier core.Tier, taskID string, by tierResults) (bool, bool) {
	pass, ok := by[tier][taskID]
	return pass, ok
}

func memoryFromTaskOutcomes(taskID, prompt string, by tierResults) *Memory {
	events := feedbackFromTask(taskID, memoryKeyForPrompt(prompt), by)
	if len(events) == 0 {
		return nil
	}
	m := BuildMemory(events)
	return &m
}

func feedbackFromTask(taskID, memoryKey string, by tierResults) []FeedbackEvent {
	var events []FeedbackEvent
	for _, tier := range []struct {
		core core.Tier
		ad   Tier
	}{
		{core.TierSmall, TierSmall},
		{core.TierMid, TierMid},
		{core.TierFrontier, TierFrontier},
	} {
		pass, ok := passAt(tier.core, taskID, by)
		if !ok {
			continue
		}
		ev := FeedbackEvent{Shape: memoryKey, SelectedTier: tier.ad}
		if pass {
			ev.Success = true
		} else {
			ev.Failed = true
		}
		events = append(events, ev)
	}
	return events
}
