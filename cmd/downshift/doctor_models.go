// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/models"
)

// checkAvailableModels says whether a real source lists the models a spawn can
// use. The catalog stand-in or no list is a warning: routing then holds or
// rests on the stand-in only.
func checkAvailableModels(add func(id, status, detail, fix string), env doctorEnv, h string) {
	const id = "available models"
	fix := "downshift models discover --harness " + h
	if env.Cat == nil {
		add(id, doctorWarn, "no catalog loaded", fix)
		return
	}
	l, err := models.Resolve(h, env.Cat, models.Options{Now: env.Now()})
	switch {
	case err != nil || len(l.Models) == 0:
		add(id, doctorWarn, "none listed; model choices are held until the harness sends a list", fix)
	case l.Fallback:
		add(id, doctorWarn, fmt.Sprintf("only the catalog stand-in (%d models), not a list from the harness", len(l.Models)), fix)
	default:
		untiered := 0
		for _, m := range l.Models {
			if m.Untiered {
				untiered++
			}
		}
		detail := fmt.Sprintf("%d from %s", len(l.Models), l.Models[0].Source)
		if untiered > 0 {
			detail += fmt.Sprintf(" (%d untiered)", untiered)
		}
		add(id, doctorOK, detail, "")
	}
}
