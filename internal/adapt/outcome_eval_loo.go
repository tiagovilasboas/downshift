// Copyright (c) 2026 Tiago de Carvalho Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package adapt

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/outcome"
)

// TierPrice is the operator-supplied price of one tier, USD per 1M tokens.
// The recorded outcome runs use models outside the embedded catalog, so the
// evaluator never guesses a price: the caller must state it.
type TierPrice struct {
	InputM  float64
	OutputM float64
}

// CostModel prices one attempt as a fixed reference request on a tier.
// A fixed reference keeps the comparison between policies about routing
// choices, not about per-task token variance.
type CostModel struct {
	Prices       [3]TierPrice // indexed by Tier (small, mid, frontier)
	RefInputTok  int
	RefOutputTok int
}

// Attempt returns the USD cost of one attempt on tier t.
func (c CostModel) Attempt(t Tier) float64 {
	p := c.Prices[clampTier(t)]
	return float64(c.RefInputTok)*p.InputM/1e6 + float64(c.RefOutputTok)*p.OutputM/1e6
}

// Validate rejects a model that cannot produce a meaningful comparison.
func (c CostModel) Validate() error {
	if c.RefInputTok < 0 || c.RefOutputTok < 0 || c.RefInputTok+c.RefOutputTok == 0 {
		return fmt.Errorf("adapt: reference request must have tokens")
	}
	for i, p := range c.Prices {
		if p.InputM < 0 || p.OutputM < 0 || p.InputM+p.OutputM == 0 {
			return fmt.Errorf("adapt: tier %d has no price", i)
		}
	}
	return nil
}

// CostRow is one routing policy evaluated on recorded runs.
type CostRow struct {
	Name      string
	Total     int
	FirstPass int // passed at the tier the policy chose, no escalation
	Completed int // passed at that tier or after escalating upward
	Attempts  int
	Cost      float64
}

// PerCompleted is total cost divided by completed tasks. It is 0 when nothing
// completed, so callers must check Completed first.
func (r CostRow) PerCompleted() float64 {
	if r.Completed == 0 {
		return 0
	}
	return r.Cost / float64(r.Completed)
}

type taskShape struct {
	id     string
	key    string
	events []FeedbackEvent
}

func collectShapes(tasks []outcome.Task, by tierResults) []taskShape {
	var out []taskShape
	for _, t := range tasks {
		key := memoryKeyForPrompt(t.Prompt)
		if key == "" {
			continue
		}
		out = append(out, taskShape{id: t.ID, key: key, events: feedbackFromTask(t.ID, key, by)})
	}
	return out
}

// looMemory builds memory from every task except index skip.
func looMemory(shapes []taskShape, skip int) Memory {
	var events []FeedbackEvent
	for i, s := range shapes {
		if i == skip {
			continue
		}
		events = append(events, s.events...)
	}
	return BuildMemory(events)
}

// EvalSharedShapeMemoryLOO is EvalSharedShapeMemory with leave-one-task-out:
// the memory used for a task never contains that task's own outcomes. This is
// the honest version of the shared-memory figure, because EvalSharedShapeMemory
// scores each task against a memory that already includes it.
func EvalSharedShapeMemoryLOO(tasks []outcome.Task, runs []outcome.Run) (OutcomeEvalReport, error) {
	by, err := indexRuns(runs)
	if err != nil {
		return OutcomeEvalReport{}, err
	}
	shapes := collectShapes(tasks, by)
	prompts := promptByID(tasks)
	var rep OutcomeEvalReport
	for i, s := range shapes {
		mem := looMemory(shapes, i)
		baseline := core.ClassifyWithSemantic(prompts[s.id]).Complexity.Tier()
		adjusted := AdjustTier(Tier(baseline), s.key, &mem)
		bPass, okB := passAt(baseline, s.id, by)
		aPass, okA := passAt(core.Tier(adjusted), s.id, by)
		if !okB || !okA {
			return OutcomeEvalReport{}, fmt.Errorf("task %s: missing tier run data", s.id)
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

// EvalCost compares routing policies by cost per completed task. A failed
// attempt escalates one tier up until frontier; a task is completed when any
// attempt passes. Policies:
//
//	classifier        start at the classifier tier
//	adapt_in_sample   start at the in-sample shared-memory tier (optimistic)
//	adapt_loo         start at the leave-one-task-out memory tier
//	small_first       start at small
//	frontier_only     start at frontier
func EvalCost(tasks []outcome.Task, runs []outcome.Run, cm CostModel) ([]CostRow, error) {
	if err := cm.Validate(); err != nil {
		return nil, err
	}
	by, err := indexRuns(runs)
	if err != nil {
		return nil, err
	}
	shapes := collectShapes(tasks, by)
	prompts := promptByID(tasks)
	global := BuildMemory(allEvents(shapes))

	names := []string{"classifier", "adapt_in_sample", "adapt_loo", "small_first", "frontier_only"}
	rows := make([]CostRow, len(names))
	for i, n := range names {
		rows[i].Name = n
	}
	for i, s := range shapes {
		baseline := Tier(core.ClassifyWithSemantic(prompts[s.id]).Complexity.Tier())
		loo := looMemory(shapes, i)
		starts := []Tier{
			baseline,
			AdjustTier(baseline, s.key, &global),
			AdjustTier(baseline, s.key, &loo),
			TierSmall,
			TierFrontier,
		}
		for p, start := range starts {
			if err := scoreAttempts(&rows[p], s.id, start, by, cm); err != nil {
				return nil, err
			}
		}
	}
	return rows, nil
}

func scoreAttempts(row *CostRow, id string, start Tier, by tierResults, cm CostModel) error {
	row.Total++
	for t := clampTier(start); t <= TierFrontier; t++ {
		pass, ok := passAt(core.Tier(t), id, by)
		if !ok {
			return fmt.Errorf("task %s: missing tier run data", id)
		}
		row.Attempts++
		row.Cost += cm.Attempt(t)
		if pass {
			if t == clampTier(start) {
				row.FirstPass++
			}
			row.Completed++
			return nil
		}
	}
	return nil
}

func allEvents(shapes []taskShape) []FeedbackEvent {
	var out []FeedbackEvent
	for _, s := range shapes {
		out = append(out, s.events...)
	}
	return out
}

func promptByID(tasks []outcome.Task) map[string]string {
	m := make(map[string]string, len(tasks))
	for _, t := range tasks {
		m[t.ID] = t.Prompt
	}
	return m
}
