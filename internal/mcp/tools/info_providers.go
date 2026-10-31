// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"encoding/json"
	"errors"
)

const providersSchema = `{"type":"object","properties":{"harness":{"type":"string","description":"Harness id, for example the one running you."},"models":{"type":"array","items":{"type":"string","maxLength":128},"maxItems":64,"description":"Optional: the model ids your own harness shows. Stored only as a hint; detection wins."}},"required":["harness"],"additionalProperties":false}`

const providersDesc = "Lists the models detected for the given harness right now, each with its tier, reasoning efforts, where Downshift learned it and an untiered flag. Call it before every subagent spawn (the Agent or Task tool), not once per session: it is the only step that shows the live models before you choose one. Fallback is true when only the catalog stand-in is known. You MAY pass the model ids your own harness shows in models: they are stored only as a hint and never override detection."

// ReportOutcome is what the store did with the ids the caller reported.
type ReportOutcome struct {
	Accepted int    `json:"accepted"`
	Written  bool   `json:"written"`
	Reason   string `json:"reason"`
}

// Reporter stores model ids the orchestrator sees for a harness. The store
// decides what is kept; the ids are hints and never task text.
type Reporter interface {
	Report(ctx context.Context, harness string, ids []string) (ReportOutcome, error)
}

// providersData is the providers result: the model list and, when the caller
// reported ids, what happened to them.
type providersData struct {
	ModelList
	Reported any `json:"reported,omitempty"`
}

// providersNext is the step after choosing: the hint the spawn hook reads, and the dry run.
const providersNext = "For mechanical work (a search, a read, a rename, running one command and reporting it, or a small well-specified edit), start the spawn's prompt with [tier=small]; the hint only moves down, a pinned model or a declared policy wins, and you choose by tier, not by name. Call apply to check a choice first."

func (d Deps) providersTool() Tool {
	fields := []string{"harness", "models"}
	t := d.build("providers", providersDesc, providersSchema, []string{"harness"}, nil)
	t.Handler = func(ctx context.Context, raw json.RawMessage) (Result, error) {
		rest, ids, err := splitModels(raw, fields)
		if err != nil {
			return Result{}, err
		}
		args, err := parseArgs(rest, fields)
		if err != nil {
			return Result{}, err
		}
		out, err := d.providers(ctx, args["harness"], ids)
		if err != nil {
			return Result{}, err
		}
		return d.envelope(ctx, out)
	}
	return t
}

func (d Deps) providers(ctx context.Context, harness string, ids []string) (outcome, error) {
	if err := d.checkHarness(harness); err != nil {
		return outcome{}, err
	}
	var reported any
	if len(ids) > 0 {
		reported = d.report(ctx, harness, ids)
	}
	if d.Models == nil {
		return outcome{}, errors.New("detected models are not available")
	}
	list, err := d.Models.Available(ctx, harness)
	if err != nil {
		return outcome{}, errors.New("could not read the models for this harness")
	}
	if list.Models == nil {
		list.Models = []Model{}
	}
	return outcome{data: providersData{ModelList: list, Reported: reported}, harnesses: []string{harness}, shrink: shrinkModels, next: providersNext}, nil
}

// report hands the ids to the Reporter; it never fails the call.
func (d Deps) report(ctx context.Context, harness string, ids []string) any {
	unavailable := map[string]bool{"unavailable": true}
	if d.Reporter == nil {
		return unavailable
	}
	res, err := d.Reporter.Report(ctx, harness, ids)
	if err != nil {
		return unavailable
	}
	return res
}

func shrinkModels(data any) (any, bool) {
	p, ok := data.(providersData)
	if !ok {
		return nil, false
	}
	var cut bool
	p.Models, cut = halve(p.Models)
	return p, cut
}
