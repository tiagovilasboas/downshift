// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// downshift-agents: live observability for Claude Code subagents, task progress
// and Downshift's Compactor and routing figures. Observation only: every hook
// passes its event on unchanged, and nothing here calls a model.
//
// Downshift by Tiago Vilas Boas
// https://github.com/tiagovilasboas/downshift

import type { Register } from 'claude-code'

import { observeAgents } from './features/agents/observe'
import { drawBand } from './features/band/band'
import { registerCommand } from './features/command/ds-agents'
import { drawPane } from './features/dashboard/pane'
import { observeTasks } from './features/tasks/observe'

export const register: Register = on => {
  registerCommand(on)
  observeAgents(on)
  observeTasks(on)
  drawBand(on)
  drawPane(on)
}
