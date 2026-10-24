// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Agents feature: the roster of this session's subagents and the pane that lists them.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { On } from 'claude-code'
import { registerAgentsCommand } from './command'
import { registerPane } from './Pane'
import { registerRoster } from './track'

export { AGENTS_COMMAND, PANE_ID } from './command'

export { formatTokens, readRoster, runningCount } from '../../shared/roster'
export type { Roster, RosterRow } from '../../shared/roster'

export function registerAgents(on: On): void {
  registerRoster(on)
  registerAgentsCommand(on)
  registerPane(on)
}
