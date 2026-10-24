// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// downshift-progress: router only. Registers the slash command at session start
// and hands each domain folder its hooks. No feature logic lives here.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { Register } from 'claude-code'

import { AGENTS_COMMAND, registerAgents } from './features/agents'
import { registerBand } from './features/band'
import { declareFlowTools, registerFlow } from './features/flow'
import { COMMAND, registerStatus } from './features/status'

export const register: Register = on => {
  on('session.start', async ($, e, next) => {
    await $.command.register(COMMAND)
    await $.command.register(AGENTS_COMMAND)
    await declareFlowTools($)
    return next(e)
  })
  registerStatus(on)
  registerBand(on)
  registerAgents(on)
  registerFlow(on)
}
