// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"errors"
)

const applySchema = `{"type":"object","properties":{` +
	`"harness":{"type":"string","description":"Harness id."},` +
	`"model":{"type":"string","description":"The model you chose for the spawn."},` +
	`"effort":{"type":"string","description":"Reasoning effort you chose."},` +
	`"tier":{"type":"string","enum":["small","mid","frontier"],"description":"Tier hint."},` +
	`"agent_type":{"type":"string","description":"Agent type of the spawn."},` +
	`"dir":{"type":"string","description":"Project directory holding the policy file."}},` +
	`"required":["harness"],"additionalProperties":false}`

func (d Deps) applyTool() Tool {
	return d.build("apply",
		"Asks what Downshift would apply to a spawn you are about to make: given the model and effort you chose, it returns the selected model, tier, effort, verdict and reason under the policy the user declared. Call it before spawning a subagent. It never takes task text.",
		applySchema, []string{"harness", "model", "effort", "tier", "agent_type", "dir"},
		func(ctx context.Context, a map[string]string) (outcome, error) {
			if err := d.checkHarness(a["harness"]); err != nil {
				return outcome{}, err
			}
			if d.Applier == nil {
				return outcome{}, errors.New("apply is not available")
			}
			res, err := d.Applier.Apply(ctx, ApplyRequest{
				Harness: a["harness"], Model: a["model"], Effort: a["effort"],
				Tier: a["tier"], AgentType: a["agent_type"], Dir: a["dir"],
			})
			if err != nil {
				return outcome{}, errors.New("could not work out what to apply")
			}
			return outcome{data: res, harnesses: []string{a["harness"]}}, nil
		})
}
