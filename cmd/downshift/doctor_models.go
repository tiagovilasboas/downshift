// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/core"
	"github.com/tiagovilasboas/downshift/internal/models"
)

// checkAvailableModels says whether a real source lists the models a spawn can
// use. For a harness that sends no list (SessionFromCatalog) the catalog
// stand-in is the design, not a gap, so it is reported as ok. A harness that
// needs a list warns when it has neither a fresh discovery entry nor a file
// list, naming the command that fixes it, and a harness whose explicit list
// has no native-named (writable) target also warns.
func checkAvailableModels(add func(id, status, detail, fix string), env doctorEnv, h string) {
	const id = "available models"
	fix := "downshift models discover --harness " + h
	if env.Cat == nil {
		add(id, doctorWarn, "no catalog loaded", fix)
		return
	}
	l, err := models.Resolve(h, env.Cat, models.Options{Now: env.Now()})
	caps, _ := core.CapsFor(h)
	switch {
	case err != nil || len(l.Models) == 0:
		add(id, doctorWarn, "no detected list and no file list; model choices are held until the harness sends one", fix)
	case l.Fallback && caps.SessionFromCatalog:
		add(id, doctorOK, fmt.Sprintf("catalog stand-in (%d models); this harness sends no list", len(l.Models)), "")
	case l.Fallback:
		add(id, doctorWarn, fmt.Sprintf("only the catalog stand-in (%d models), not a list from the harness", len(l.Models)), fix)
	case caps.SessionFromCatalog && !hasNativeTarget(env, h, l):
		add(id, doctorWarn, fmt.Sprintf("%d listed from %s, but none has a native name the hook can write; rewrites are held", len(l.Models), l.Models[0].Source),
			"remove the explicit "+h+" list from session-models.json, or list the names the harness accepts")
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

// hasNativeTarget reports whether at least one listed model resolves to a
// catalog entry that declares a native name, the string a hook can write.
func hasNativeTarget(env doctorEnv, h string, l models.Listing) bool {
	for _, m := range l.Models {
		if cm, ok := env.Cat.LookupByID(h, m.ID); ok && cm.Native != "" {
			return true
		}
	}
	return false
}
