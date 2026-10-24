// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Test helpers: the engine beneath the mod, a spawn and an end of an agent.

import type { On } from 'claude-code'
import { mock } from 'claude-code/testing'
import type { Engine } from 'claude-code/testing'

export type World = { panes: string[]; clock: ReturnType<typeof mock.clock>; state: <T>(key: string) => T; argvs: string[][]; setPanel: (json: string | null) => void }

/** Mocks clock, session id, process and the spawn; `setPanel(null)` makes the binary fail. */
export function world($: Engine, on: On, start = 1_000_000): World {
  const clock = mock.clock(on, { now: start })
  const argvs: string[][] = []
  let panel: string | null = null
  const ids = new Map<string, string>()
  const written = new Map<string, unknown>()
  // A test hook cannot read state, so it watches the writes: the last value per key.
  on('state.set', (_$, e, next) => { written.set(e.key, e.value); return next(e) })
  on('turn.complete', () => ({ text: '' }))
  on('session.id', () => ({ value: 'S-1' }))
  const panes: string[] = []
  const opened = new Set<string>()
  on('ui.panes', () => ({ value: [...opened].map(id => ({ id, title: id, isShown: true, isFocused: false, isPlaced: true })) }))
  on('ui.open', (_$, e) => { panes.push(`open:${e.id}`); opened.add(e.id); return { value: { isPlaced: true } } })
  on('ui.close', (_$, e) => { panes.push(`close:${e.id}`); opened.delete(e.id); return { value: undefined } })
  on('process.run', (_$, e) => {
    argvs.push([...e.argv])
    if (panel === null) throw new Error('spawn ENOENT')
    return { value: { exitCode: 0, stdout: e.argv.includes('--no-report') ? withoutReport(panel) : panel, stderr: '', isStdoutTruncated: false, isStderrTruncated: false } }
  })
  on('agent.spawn', (_$, e) => {
    const agentId = ids.get(e.tool_use_id) ?? `agent-${ids.size + 1}`
    ids.set(e.tool_use_id, agentId)
    return { model: 'claude-sonnet-5-5', agentId }
  })
  return { panes, clock, state: <T>(key: string) => written.get(key) as T, argvs, setPanel: json => { panel = json } }
}

/** The binary leaves the report block out under --no-report. */
function withoutReport(json: string): string {
  const { report: _report, ...rest } = JSON.parse(json) as Record<string, unknown>
  return JSON.stringify(rest)
}

export const spawn = ($: Engine, toolUseId: string, description: string) =>
  $.agent.spawn({
    tool_use_id: toolUseId, prompt: 'p', description, subagentType: 'general-purpose',
    provider: 'engine', parentModel: 'claude-opus-5-5', background: false, fork: false,
  } as never)

export const complete = ($: Engine, agentId: string, reason: 'answer' | 'error' | 'aborted' = 'answer') =>
  $.turn.complete({ answer: '', durationMs: 5, isAborted: false, turnId: `t-${agentId}`, agentId, reason } as never)

