// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The subagent card of the dashboard: the orchestrator (the main session) as the
// root row, then one line per agent under it with the task it was given, one
// dim rule between agents. A child is drawn under its parent
// only when agent.spawn named the parent loop and that parent is listed;
// otherwise it hangs directly under the orchestrator. A model is called
// "observed" only with evidence (see modelClaim).

import { isLive } from '../agents/stale'
import type { Agent, DownshiftSnapshot } from '../../../types'
import { modelClaim, orchestratorOf, spawnOf, taskOf, visibleAgents, workTokens } from '../metrics/normalize'
import type { ModelClaim, Orchestrator } from '../metrics/normalize'
import { ACCENT, MUTED, elapsed, fit, modelName, tokens } from '../../shared/ui/format'
import { lookOf } from '../../shared/ui/status'
import { card, innerColumns } from './card'
import type { Ui } from './ui'

export const MAX_ROWS = 30

type Row = { agent: Agent; depth: number }

/** Parents before children, depth-first, in spawn order. */
export function treeRows(agents: readonly Agent[]): Row[] {
  const ids = new Set(agents.map(a => a.id))
  const childrenOf = (id: string | undefined) =>
    agents.filter(a => (id === undefined ? a.parentId === undefined || !ids.has(a.parentId) : a.parentId === id))
  const out: Row[] = []
  const walk = (id: string | undefined, depth: number) => {
    for (const a of childrenOf(id)) {
      if (out.some(r => r.agent.id === a.id)) {
        continue
      }
      out.push({ agent: a, depth })
      walk(a.id, depth + 1)
    }
  }
  walk(undefined, 0)
  return out
}

export function agentsSection(
  ui: Ui,
  input: { agents: readonly Agent[]; snapshot: DownshiftSnapshot; filter: 'active' | 'all'; isExpanded: boolean; now: number },
  columns: number,
) {
  const { Box, Text } = ui
  const inner = innerColumns(columns)
  const all = visibleAgents(input.agents)
  const shown = input.filter === 'active' ? all.filter(a => isLive(a, input.now)) : all
  const rows = treeRows(shown)
  const active = all.filter(a => a.status === 'running').length
  const stale = all.filter(a => a.status === 'stale' && isLive(a, input.now)).length
  const empty = input.filter === 'active' && all.length > 0 ? 'No agent running now.' : 'None yet in this session.'
  const summary = `${active} active${stale > 0 ? ` · ${stale} stale` : ''} · ${all.length} total`

  const root = orchestratorOf(input.agents)

  return card(ui, { title: 'SUBAGENTS', summary, summaryColor: active > 0 ? ACCENT : MUTED, columns }, [
    rows.length === 0 && <Text color={MUTED}>{empty}</Text>,
    rows.length > 0 && orchestratorRow(ui, root, inner),
    ...rows.slice(0, MAX_ROWS).flatMap(({ agent, depth: treeDepth }, i) => {
      // the orchestrator is the root, so every agent hangs one level below it
      const depth = treeDepth + 1
      const claim = modelClaim(agent, spawnOf(agent, input.snapshot))
      const look = lookOf(agent.status)
      const indent = '  '.repeat(Math.min(depth, 4))
      const right = `${look.label} · ${elapsed((agent.endedAt ?? input.now) - agent.startedAt)}`
      const name = `${depth > 0 ? '└ ' : ''}${agent.type}`
      const task = taskOf(agent)
      const claimLine = ` ${claimText(claim)}`
      // one line per agent: type, then the model claim and the task (muted) in what the status leaves free
      const nameWidth = Math.min(name.length, Math.max(8, inner - indent.length - right.length - 5))
      return [
        i > 0 && (
          <Text dimColor wrap="truncate-end">
            {'─'.repeat(Math.max(1, inner))}
          </Text>
        ),
        <Box flexDirection="column">
          <Box flexDirection="row" justifyContent="space-between" gap={1}>
            <Text wrap="truncate-end">
              {indent}
              <Text color={look.color}>{look.mark} </Text>
              <Text bold>{fit(name, nameWidth)}</Text>
              <Text color={MUTED}>{claimLine}</Text>
              {task !== undefined && <Text color={MUTED}>{` · ${task}`}</Text>}
            </Text>
            <Box flexShrink={0}>
              <Text color={look.color}>{right}</Text>
            </Box>
          </Box>
          {input.isExpanded && details(ui, agent, claim, input.snapshot, indent, inner)}
        </Box>,
      ]
    }),
    rows.length > MAX_ROWS && <Text color={MUTED}>{rows.length - MAX_ROWS} more not shown</Text>,
  ])
}

/** The root row: the main session that spawned the agents, its model as far as a spawn reported it. */
function orchestratorRow(ui: Ui, root: Orchestrator, inner: number) {
  const { Box, Text } = ui
  const right = root.isActive ? 'running' : 'idle'
  const color = root.isActive ? ACCENT : MUTED
  return (
    <Box flexDirection="row" justifyContent="space-between" gap={1}>
      <Text wrap="truncate-end">
        <Text color={color}>{root.isActive ? '◆ ' : '◇ '}</Text>
        <Text bold>{fit('orchestrator', Math.max(8, inner - right.length - 4))}</Text>
        {root.model !== undefined && <Text color={MUTED}>{` ${modelName(root.model)}`}</Text>}
      </Text>
      <Box flexShrink={0}>
        <Text color={color}>{right}</Text>
      </Box>
    </Box>
  )
}

function details(ui: Ui, agent: Agent, claim: ModelClaim, snapshot: DownshiftSnapshot, indent: string, columns: number) {
  const { Text } = ui
  return detailLines(agent, claim, snapshot).map(l => (
    <Text dimColor wrap="truncate-end">
      {fit(`${indent}    ${l}`, columns)}
    </Text>
  ))
}

/** The expanded facts of one agent: type, models, tools, measured tokens and Downshift's reason. */
export function detailLines(agent: Agent, claim: ModelClaim, snapshot: DownshiftSnapshot): string[] {
  const spawn = spawnOf(agent, snapshot)
  const work = workTokens(agent)
  return [
    `type ${agent.type}${agent.runs > 1 ? ` · run ${agent.runs}` : ''}`,
    `requested ${agent.requestedModel === undefined ? 'inherit' : modelName(agent.requestedModel)}` +
      (spawn === undefined ? '' : ` · Downshift ${modelName(spawn.requested)} → ${modelName(spawn.final)}`),
    claim.basis === 'observed' ? `effective ${modelName(claim.model)} (observed)` : 'effective: not observed yet',
    `tools ${agent.tools}${agent.lastTool === undefined ? '' : ` · last ${agent.lastTool}`}`,
    work === undefined ? 'tokens: not reported yet' : `tokens ${tokens(work)} work · ${tokens(agent.tokens?.cacheRead ?? 0)} cache read (measured)`,
    spawn?.reason === undefined ? '' : `why: ${spawn.reason}`,
  ].filter(Boolean)
}

/** The evidence behind a model claim as one muted mark: ✓ observed, ~ decided, ? requested, ↑ inherited. */
export function claimMark(c: ModelClaim): string {
  return { observed: '✓', decided: '~', requested: '?', inherited: '↑' }[c.basis]
}

export function claimText(c: ModelClaim): string {
  const name = modelName(c.model)
  return c.basis === 'observed' ? name : `${name} (${c.basis})`
}
