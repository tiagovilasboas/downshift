// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package setup

import (
	"fmt"

	"github.com/tiagovilasboas/downshift/internal/contextopt"
)

// Run is `downshift setup` after argument parsing: it selects the targets,
// plans every file, confirms, writes, and prints the summary. It returns the
// process exit code.
func Run(o Options) int {
	targets, code := Select(o)
	if code != 0 {
		return code
	}
	if o.Undo {
		code := Undo(o, targets)
		if mcpCode := mcpUndo(o, targets); mcpCode != 0 {
			code = mcpCode
		}
		return code
	}

	plans := planTargets(&o, targets)

	if p, ok := PlanPolicyInit(o); ok {
		plans = append(plans, p)
	}
	if p, ok := planWorkers(o); ok {
		plans = append(plans, p)
	}
	plans = RefuseLinkedPlans(o.LinkBases(), plans)
	mcpFiles, mcpSteps := planMcp(o, targets)
	failed, proceed := ApplyPlans(o, plans)
	if runSetupMcp(o, mcpFiles, mcpSteps, proceed, failed) {
		failed = true
	}
	if !failed && !o.Undo {
		failed = pruneBinaries(o, plans)
	}
	if !proceed {
		if failed {
			return 1
		}
		return 0
	}
	// Ensure native context optimization is enabled by default.
	if optCfg, err := contextopt.LoadConfig(); err == nil {
		if !optCfg.Enabled {
			optCfg = contextopt.DefaultConfig()
			_ = contextopt.SaveConfig(optCfg)
		}
	}
	// Run discovery for harnesses with a source so sessions don't stay on handwritten lists.
	if o.DiscoverModels != nil {
		o.DiscoverModels(o)
	}
	// Say which harnesses still lack a detected model list (read-only: nothing is written).
	if o.EnsureSessionModels != nil {
		o.EnsureSessionModels(o, targets)
	}

	fmt.Fprint(o.Out, Summary(plans))
	fmt.Fprintln(o.Out, "Undo with: downshift setup --undo")
	if o.ProjectDir == "" {
		fmt.Fprintln(o.Out, "Send read-only Explore spawns to the small tier in a project: downshift policy init (shows the file first)")
	}
	if failed {
		return 1
	}
	return 0
}
