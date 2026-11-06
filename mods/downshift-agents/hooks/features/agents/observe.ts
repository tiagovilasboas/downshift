// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Observes subagents through engine events and feeds the roster. Every hook
// passes the event on unchanged (`next(e)`) and only then records what it saw:
// the mod never rewrites a model, a prompt or a result.

import type { On } from 'claude-code'

import { showPane } from '../dashboard/pane'
import { refresh } from '../downshift/refresh'
import { toolUseKey } from '../downshift/panel'
import { ensureSweeper } from '../liveness/sweep'
import { updateAgents } from '../../shared/state/atoms'
import { spawnDenied, spawnSettled, spawnStarted, toolUsed, turnCompleted } from './roster'

const MAX_LABEL = 80

export function observeAgents(on: On): void {
  on('agent.spawn', async ($, e, next) => {
    // A workflow's agents carry ids no list names; they are not subagents here.
    if (e.workflow !== undefined) {
      return next(e)
    }
    const now = await $.clock.now()
    const key = e.tool_use_id === '' ? undefined : await toolUseKey(e.tool_use_id)
    await updateAgents($, list =>
      spawnStarted(list, {
        toolUseId: e.tool_use_id,
        toolUseKey: key,
        label: e.description.slice(0, MAX_LABEL),
        type: e.subagentType,
        parentId: e.parentAgentId,
        requestedModel: e.model,
        parentModel: e.parentModel,
        resumeId: e.resume?.agentId,
        now,
      }),
    )
    ensureSweeper($)
    const started = await next(e)
    await updateAgents($, list =>
      started.deny !== undefined
        ? spawnDenied(list, e.tool_use_id)
        : started.agentId === undefined
          ? [...list]
          : spawnSettled(list, e.tool_use_id, started.agentId, started.model),
    )
    if (started.deny === undefined) {
      await showPane($)
    }
    void refresh($)

    return started
  })

  on('tool.call', async ($, e, next) => {
    const ran = await next(e)
    const agentId = e.agentId
    if (agentId !== undefined) {
      const now = await $.clock.now()
      await updateAgents($, list => toolUsed(list, agentId, String(e.tool), now))
    }

    return ran
  })

  on('turn.complete', async ($, e, next) => {
    const done = await next(e)
    const agentId = e.agentId
    if (agentId === undefined) {
      void refresh($, true)
      return done
    }
    const now = await $.clock.now()
    const u = e.usage
    await updateAgents($, list =>
      turnCompleted(list, {
        agentId,
        turnId: e.turnId,
        reason: e.reason,
        model: u?.model,
        usage:
          u === undefined
            ? undefined
            : {
                input: u.input_tokens,
                output: u.output_tokens,
                cacheRead: u.cache_read_input_tokens,
                cacheWrite: u.cache_creation_input_tokens,
              },
        now,
      }),
    )
    void refresh($)

    return done
  })
}
