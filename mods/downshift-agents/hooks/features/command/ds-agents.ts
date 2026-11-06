// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The /ds-agents slash command: opens or closes the dashboard and prints a
// plain-text summary. Named apart from Downshift's own /downshift commands.

import type { EngineInterface, On } from 'claude-code'

import { refresh } from '../downshift/refresh'
import { report } from './report'
import { agentCounts, reductionPercent } from '../metrics/normalize'
import { progressOf } from '../tasks/board'
import { readAll } from '../../shared/state/atoms'
import { bytes, tokens } from '../../shared/ui/format'
import { PANE_ID, PANE_TITLE } from '../dashboard/pane'
import { ESTIMATE_NOTE, ROUTING_NOTE } from '../dashboard/metrics-section'

export const COMMAND = 'ds-agents'

export const HELP = [
  '/ds-agents            open the Downshift Agents dashboard',
  '/ds-agents close      close it',
  '/ds-agents summary    tasks, agents, Compactor and routing as text',
  '/ds-agents report     the downshift report: Compactor per family and harness, routing',
  '/ds-agents help       this list',
  'In the pane: show all or active agents, expand details, refresh Downshift, hide the band.',
].join('\n')

export function registerCommand(on: On): void {
  on('session.start', async ($, e, next) => {
    const started = await next(e)
    await $.command.register({ name: COMMAND, description: 'Downshift Agents: open the live dashboard, or summary, report, close, help' })
    void refresh($, true)

    return started
  })

  on('command.run', { command: COMMAND }, async ($, e) => {
    const verb = e.args.trim().toLowerCase()
    if (verb === 'help') {
      return { text: HELP }
    }
    if (verb === 'close') {
      await $.ui.close({ id: PANE_ID })
      return { text: 'Downshift Agents closed.' }
    }
    if (verb === 'summary') {
      await refresh($, true)
      return { text: await summary($) }
    }
    if (verb === 'report') {
      return { text: await report($) }
    }
    if (verb !== '' && verb !== 'open') {
      return { text: `Unknown option "${verb}".\n${HELP}` }
    }
    const opened = await $.ui.open({ id: PANE_ID, title: PANE_TITLE })
    void refresh($, true)
    return { text: opened.isPlaced ? 'Downshift Agents opened.' : `This surface draws no pane; try /${COMMAND} summary.` }
  })
}

export async function summary($: EngineInterface): Promise<string> {
  const { agents, board, downshift: snapshot } = await readAll($)
  const p = progressOf(board)
  const a = agentCounts(agents)
  const lines = [
    p.total === 0 ? 'Tasks: none reported' : `Tasks: ${p.completed}/${p.total} completed (${p.percent ?? 0}%), ${p.running} running, ${p.blocked} blocked${p.stale > 0 ? `, ${p.stale} stale` : ''}`,
    `Agents: ${a.active} active, ${a.total} total, ${a.failed} failed${a.stale > 0 ? `, ${a.stale} stale` : ''}`,
  ]
  if (snapshot.status !== 'ok') {
    lines.push(snapshot.status === 'unavailable' ? `Downshift: unavailable (${snapshot.reason})` : 'Downshift: not read yet')
    return lines.join('\n')
  }
  const c = snapshot.compactor
  if (c !== undefined) {
    const pct = reductionPercent(c)
    lines.push(
      `Compactor (${c.scope}): ${c.compacted}/${c.commands} runs compacted, raw ${bytes(c.rawBytes)}, reduced ${bytes(c.savedBytes)}${pct === undefined ? '' : ` (${pct}%)`}, ~${tokens(c.estimatedTokens)} tokens saved*`,
    )
    if (c.families.length > 0) {
      lines.push(`Top families: ${c.families.map(f => `${f.family} ~${tokens(f.avoidedTokens)}*`).join(', ')}`)
    }
  }
  const r = snapshot.routing
  lines.push(
    `Routing (this session): model changed ${r.modelSwitched}, held ${r.held}, agents below the session model ${r.belowChildren}/${r.children}, billable below the session model ${tokens(r.belowBillable)} of ${tokens(r.billable)} tokens`,
    `  ${ROUTING_NOTE}`,
    ESTIMATE_NOTE,
  )
  return lines.join('\n')
}
