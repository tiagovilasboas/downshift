// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"
)

// planTargets plans the files of every selected target, in the order setup
// writes them. It takes the options by pointer because the plugin step records
// which harness plugins are active, and the rest of the run reads that record.
func planTargets(o *Options, targets []Target) []Plan {
	var plans []Plan
	extra := map[string][]string{}
	for _, t := range targets {
		if t.File == "" && len(t.Files) == 0 {
			fmt.Fprintf(o.Out, "%s: detected, not wired by setup\n  %s\n\n", t.Name, t.Manual)
			if p, ok := planSteering(*o, t); ok && !o.NoSteering {
				plans = append(plans, p)
			}
			continue
		}
		// A hook file or a bridge file setup cannot write stops the target: the
		// error says nothing was written for it, so its instructions file stays too.
		var targetErr error
		if t.File != "" {
			if active, notes := ensurePlugin(*o, t); active || len(notes) > 0 {
				if o.PluginActive == nil {
					o.PluginActive = map[string]bool{}
				}
				o.PluginActive[t.Name] = active
				extra[t.Name] = notes
			}
			p := planTarget(*o, t)
			p.Notes = append(extra[t.Name], p.Notes...)
			plans = append(plans, p.Companions...)
			plans = append(plans, p)
			targetErr = p.Err
			if reg, ok := planCrewRegistry(*o, t); ok {
				plans = append(plans, reg)
			}
		}
		files := PlanHarnessFiles(*o, t)
		for _, f := range files {
			if f.Err != nil {
				targetErr = f.Err
			}
		}
		plans = append(plans, files...)
		// A hook file setup cannot read stops the target: the error says
		// nothing was written for it, so its instructions file stays too.
		if sp, ok := planSteering(*o, t); ok && targetErr == nil && !o.NoSteering {
			plans = append(plans, sp)
		}
		if !o.NoSkills {
			plans = append(plans, planSkills(*o, t)...)
		}
		plans = append(plans, planMods(*o, t)...)
		plans = append(plans, planModProfiles(*o, t)...)
	}
	return plans
}
