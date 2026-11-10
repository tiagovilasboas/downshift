// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"slices"

	"github.com/tiagovilasboas/downshift-ai/internal/console/setupwizard"
)

// essentialComponents are the parts that cut tokens on their own: the hooks
// (routing and the compactor), the plugin that carries them in Claude Code,
// the steering that tells other harnesses to compact, and the project policy
// that runs Explore on the small model.
var essentialComponents = []string{"hooks", "plugin", "steering", "policy"}

// wizardPresets offers Essential first (what Enter picks), Complete (every
// component that is on by default) and Custom (the full list). A preset only
// selects components that start selected, so a flag such as --no-mcp or a
// missing project directory still holds; a destructive one is never chosen.
func wizardPresets(cs []setupwizard.Component) []setupwizard.Preset {
	var essential, complete []string
	for _, c := range cs {
		if !c.Default || c.Destructive {
			continue
		}
		complete = append(complete, c.ID)
		if slices.Contains(essentialComponents, c.ID) {
			essential = append(essential, c.ID)
		}
	}
	return []setupwizard.Preset{
		{ID: "essential", Label: "Essential (recommended)", Components: essential,
			Hint: "right model per subagent, shorter test output; nothing else"},
		{ID: "complete", Label: "Complete", Components: complete, AskChoice: true,
			Hint: "Essential plus the MCP server, skills, the live pane and session policy"},
		{ID: "custom", Label: "Custom", Custom: true, Hint: "pick each component with space"},
	}
}
