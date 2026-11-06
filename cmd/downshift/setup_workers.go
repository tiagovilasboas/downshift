// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

package main

import "github.com/tiagovilasboas/downshift/internal/console/setupwizard"

// workersChoice is the wizard question. Read comes first so Enter picks the
// recommended answer; write is last and never preselected.
func workersChoice() *setupwizard.Choice {
	return &setupwizard.Choice{
		ID:    "workers",
		Title: "Worker tools: let your agent delegate tasks to other coding CLIs",
		Items: []setupwizard.ChoiceItem{
			{ID: "read", Label: "read (recommended)", Hint: "workers may read and answer; they cannot change files (Claude and Codex workers)"},
			{ID: "off", Label: "off", Hint: "no worker tools"},
			{ID: "write", Label: "write", Hint: "workers edit files in their own git worktree; never merged"},
		},
	}
}
