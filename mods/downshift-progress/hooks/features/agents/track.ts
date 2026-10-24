// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The hooks that keep the roster: a spawn starts a row, a turn step adds its
// tokens, the end of the agent's turn finishes it. Every hook hands the event on
// and never throws, so a roster fault cannot change a spawn or a turn.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { On } from 'claude-code'
import { PROGRESS_TOOL } from '../flow'
import { toolUseKey } from '../../shared/roster/key'
import { addTokens, endRun, startRun } from '../../shared/roster/state'
import { usageTotal } from '../../shared/roster/usage'
import { syncPlanned, tierOfTask } from './planned'
import { refreshFigures, scheduleExpiry, syncTicker } from './refresh'

export function registerRoster(on: On): void {
  on('agent.spawn', async ($, e, next) => {
    const started = await next(e)
    try {
      if (started.deny === undefined && started.agentId) {
        await startRun($, {
          toolUse: e.tool_use_id,
          key: await toolUseKey(e.tool_use_id),
          agentId: started.agentId,
          description: e.description,
          type: e.subagentType,
          tier: await tierOfTask($, e.description),
          model: started.model,
          startedAt: await $.clock.now(),
          tokens: 0,
        })
        await syncPlanned($)
        await syncTicker($)
        void refreshFigures($)
      }
    } catch {
      // The spawn already started; the roster just misses the row.
    }
    return started
  })

  on('turn.step', async function* ($, e, next) {
    const result = yield* next(e)
    try {
      if (e.agentId !== undefined && result) await addTokens($, e.agentId, usageTotal(result.usage))
    } catch {
      // Tokens stay as they were.
    }
    return result
  })

  on('turn.complete', async ($, e, next) => {
    const result = await next(e)
    try {
      if (e.agentId !== undefined) {
        const row = await endRun($, e.agentId, await $.clock.now(), e.reason !== 'answer')
        if (row) {
          scheduleExpiry($)
          await syncTicker($)
          void refreshFigures($)
        }
      }
    } catch {
      // The row stays running until the next turn event or a reload.
    }
    return result
  })

  // The flow tools write the flow; the planned list follows it.
  on('tool.call', { tool: PROGRESS_TOOL }, async ($, e, next) => {
    const result = await next(e)
    try {
      await syncPlanned($)
    } catch {
      // Planned catches up at the next spawn.
    }
    return result
  })
}
