// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The two tools of the flow: `progress` (main session) and `step` (workers).
// A call never throws: bad input or a wrong caller answers a text result.
// The step-brief prompt injection is opt-in and not part of this module.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { EngineInterface, On } from 'claude-code'
import { PLUGIN } from '../../shared/data/plugin'
import { MAX_AFTER, MAX_NOTE, MAX_STEP_TOTAL, MAX_TITLE, MAX_TOTAL, PHASES, TIERS, parseProgress, parseStep } from './bounds'
import { matchTask, mergeProgress, mergeStep } from './flow'
import { readFlow, readStepOf, writeFlow, writeStep } from './state'

export const PROGRESS_TOOL = `mcp__${PLUGIN}__progress`
export const STEP_TOOL = `mcp__${PLUGIN}__step`

const text = (message: string, isError = false) => ({
  result: { content: [{ type: 'text', text: message }], ...(isError ? { isError: true } : {}) },
})

export const PROGRESS_SPEC = {
  name: 'progress',
  description:
    'Main session only: register or update the flow shown in the progress band. Fields left out keep their previous value; a call with a new title starts a new flow.',
  isDeferred: false,
  inputSchema: {
    type: 'object',
    properties: {
      title: { type: 'string', maxLength: MAX_TITLE },
      total: { type: 'integer', minimum: 0, maximum: MAX_TOTAL },
      done: { type: 'integer', minimum: 0, maximum: MAX_TOTAL },
      phase: { type: 'string', enum: [...PHASES] },
      finished: { type: 'boolean' },
      tasks: {
        type: 'array',
        maxItems: MAX_TOTAL,
        items: {
          type: 'object',
          required: ['title'],
          properties: {
            title: { type: 'string', maxLength: MAX_TITLE },
            tier: { type: 'string', enum: [...TIERS] },
            after: { type: 'array', maxItems: MAX_AFTER, items: { type: 'string', maxLength: MAX_TITLE } },
          },
        },
      },
    },
  },
}

export const STEP_SPEC = {
  name: 'step',
  description: 'Workers only: report your own progress (done of total steps and a short note).',
  isDeferred: false,
  inputSchema: {
    type: 'object',
    properties: {
      total: { type: 'integer', minimum: 0, maximum: MAX_STEP_TOTAL },
      done: { type: 'integer', minimum: 0, maximum: MAX_STEP_TOTAL },
      note: { type: 'string', maxLength: MAX_NOTE },
    },
  },
}

/**
 * Declares both tools. Called from the router's one `session.start` hook: the
 * engine refuses a second unmatched `session.start` hook in a module. Fails open.
 */
export async function declareFlowTools($: Pick<EngineInterface, 'tool'>): Promise<void> {
  try {
    await $.tool.register(PROGRESS_SPEC)
    await $.tool.register(STEP_SPEC)
  } catch {
    // The mod still loads without its tools.
  }
}

export const registerFlowTools = (on: On): void => {
  on('tool.call', { tool: PROGRESS_TOOL }, async ($, e) => {
    try {
      if (e.agentId !== undefined) return text('progress is for the main session; this call from a subagent was ignored.')
      const parsed = parseProgress(e)
      if (!parsed.ok) return text(`progress: ${parsed.error}`, true)
      const merged = mergeProgress(await readFlow($), parsed.value)
      if ('error' in merged) return text(`progress: ${merged.error}`, true)
      await writeFlow($, merged.flow, merged.isNew)
      const f = merged.flow
      return text(`${f.title}: ${f.done}/${f.total}${f.finished ? ' (finished)' : ''}`)
    } catch {
      return text('progress: could not be applied.', true)
    }
  })

  on('tool.call', { tool: STEP_TOOL }, async ($, e) => {
    try {
      const agentId = e.agentId
      if (agentId === undefined) return text('step is for workers; this call from the main session was ignored.')
      const parsed = parseStep(e)
      if (!parsed.ok) return text(`step: ${parsed.error}`, true)
      const description = (await $.agent.list()).find(a => a.id === agentId)?.description
      const task = matchTask(await readFlow($), description)
      const merged = mergeStep(await readStepOf($, agentId), parsed.value, task)
      if ('error' in merged) return text(`step: ${merged.error}`, true)
      await writeStep($, agentId, merged)
      return text(`${merged.done}/${merged.total}${merged.task === null ? '' : ` (task ${merged.task})`}`)
    } catch {
      return text('step: could not be applied.', true)
    }
  })
}
