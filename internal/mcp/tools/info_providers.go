// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package tools

import (
	"context"
	"errors"
)

const providersSchema = `{"type":"object","properties":{"harness":{"type":"string","description":"Harness id, for example the one running you."}},"required":["harness"],"additionalProperties":false}`

func (d Deps) providersTool() Tool {
	return d.build("providers",
		"Lists the models the given harness can run right now, each with its tier, reasoning efforts, where Downshift learned it and an untiered flag. Call it before choosing a model for a delegation. fallback is true when only the catalog stand-in is known.",
		providersSchema, []string{"harness"},
		func(ctx context.Context, a map[string]string) (outcome, error) {
			if err := d.checkHarness(a["harness"]); err != nil {
				return outcome{}, err
			}
			if d.Models == nil {
				return outcome{}, errors.New("detected models are not available")
			}
			list, err := d.Models.Available(ctx, a["harness"])
			if err != nil {
				return outcome{}, errors.New("could not read the models for this harness")
			}
			if list.Models == nil {
				list.Models = []Model{}
			}
			return outcome{data: list, harnesses: []string{a["harness"]}, shrink: shrinkModels}, nil
		})
}

func shrinkModels(data any) (any, bool) {
	l, ok := data.(ModelList)
	if !ok {
		return nil, false
	}
	var cut bool
	l.Models, cut = halve(l.Models)
	return l, cut
}
