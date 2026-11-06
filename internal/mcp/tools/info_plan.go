// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/tiagovilasboas/downshift/internal/plan"
	"github.com/tiagovilasboas/downshift/internal/playbook"
)

const planSchema = `{"type":"object","properties":{"goal":{"type":"string","description":"One sentence: what the plan achieves."},"parts":{"type":"array","description":"The slices of work, as for downshift plan on stdin.","items":{"type":"object","properties":{"id":{"type":"string"},"title":{"type":"string"},"agent":{"type":"string"},"points":{"type":"integer"},"depends_on":{"type":"array","items":{"type":"string"}},"files":{"type":"array","items":{"type":"string"}},"worker":{"type":"string","description":"native (default), claude or codex."},"playbook":{"type":"string"}},"required":["id","title","agent","points"],"additionalProperties":false}}},"required":["goal","parts"],"additionalProperties":false}`

// planResult is the data of the plan tool. An invalid plan sets Valid false and
// lists every problem; the plan fields are then empty.
type planResult struct {
	Valid    bool       `json:"valid"`
	Problems []string   `json:"problems,omitempty"`
	Plan     *plan.Plan `json:"plan,omitempty"`
	Markdown string     `json:"markdown,omitempty"`
}

// planTool validates and fills a plan through plan.Resolve, the code path
// `downshift plan` uses. The plan text is read, answered and dropped: nothing
// is stored.
func (d Deps) planTool() Tool {
	return Tool{
		Name:        "plan",
		Description: "Validates a delegation plan you wrote (goal and parts with id, title, agent, points, depends_on, files) and fills tier, wave and route for each part, with warnings and next steps wave by wave. Invalid plans return every problem. Calls no model and stores nothing.",
		InputSchema: json.RawMessage(planSchema),
		Handler: func(ctx context.Context, raw json.RawMessage) (Result, error) {
			raw = bytes.TrimSpace(raw)
			if len(raw) == 0 {
				return Result{}, errors.New("arguments must be a JSON object with goal and parts")
			}
			ids := playbook.ShippedIDs()
			if sets, err := d.sets(); err == nil {
				ids = ids[:0:0]
				for _, s := range sets {
					ids = append(ids, s.ID)
				}
			}
			var gw plan.Gateway
			if d.Gateway != nil {
				gw = d.Gateway()
			}
			p, problems := plan.Resolve(raw, gw, d.Workers.Plan(), ids)
			res := planResult{Valid: len(problems) == 0, Problems: problems}
			if res.Valid {
				res.Plan, res.Markdown = &p, p.Markdown()
				d.recordCall("plan", "", planCounts(p.Next))
			} else {
				d.recordCall("plan", "invalid", CallCounts{})
			}
			return d.envelope(ctx, outcome{data: res, harnesses: d.harnessIDs()})
		},
	}
}

// planCounts counts the resolved steps by kind.
func planCounts(steps []plan.Step) CallCounts {
	c := CallCounts{Steps: len(steps)}
	for _, s := range steps {
		if s.Kind == plan.KindDelegate {
			c.DelegateSteps++
		} else {
			c.SpawnSteps++
		}
	}
	return c
}
