// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The subagent panel where the surface draws Svg: one compact row per agent
// (bot, name, the model as far as the evidence goes, status chip, elapsed
// time) with one dim rule between rows. No progress share per agent: the engine reports none.

import type { Agent, DownshiftSnapshot } from '../../../../types'
import { modelClaim, orchestratorOf, spawnOf, taskOf, visibleAgents } from '../../metrics/normalize'
import { ACCENT, MUTED, elapsed, modelName } from '../../../shared/ui/format'
import { lookOf } from '../../../shared/ui/status'
import { agentSprite } from '../../../shared/ui/svg/sprite/agent-sprite'
import { orchestratorSprite } from '../../../shared/ui/svg/sprite/orchestrator'
import { icon } from '../../../shared/ui/svg/icons'
import { pill, rule } from '../../../shared/ui/svg/shapes'
import { MAX_ROWS, claimMark, detailLines, treeRows } from '../agents-section'
import { art, panel } from './frame'
import type { GraphicUi } from './frame'

/** Free-standing figure beside a row, px: big enough to read the tool in its hand. */
const BOT_SIZE = 24
const ORCHESTRATOR_SIZE = 28

type Input = { agents: readonly Agent[]; snapshot: DownshiftSnapshot; filter: 'active' | 'all'; isExpanded: boolean; now: number }

export function agentsPanel(ui: GraphicUi, input: Input) {
  const { Text } = ui
  const all = visibleAgents(input.agents)
  const shown = input.filter === 'active' ? all.filter(a => a.status === 'running' || a.status === 'stale') : all
  const rows = treeRows(shown)
  const active = all.filter(a => a.status === 'running').length
  const stale = all.filter(a => a.status === 'stale').length
  const empty = input.filter === 'active' && all.length > 0 ? 'No agent running now.' : 'None yet in this session.'

  return panel(ui, { title: 'SUBAGENTS', color: ACCENT, summary: `${active} active${stale > 0 ? ` · ${stale} stale` : ''} · ${all.length} total`, icon: icon('agents', 'green') }, [
    rows.length === 0 && (
      <Text size="footnote" color={MUTED}>
        {empty}
      </Text>
    ),
    rows.length > 0 && orchestratorRow(ui, input.agents),
    ...rows.slice(0, MAX_ROWS).flatMap(({ agent, depth }, i) => [
      i > 0 && art(ui, rule(), 'divider', true),
      // the orchestrator is the root, so every agent hangs one level below it
      agentRow(ui, agent, depth + 1, input),
    ]),
    rows.length > MAX_ROWS && (
      <Text size="footnote" color={MUTED}>
        {`${rows.length - MAX_ROWS} more not shown`}
      </Text>
    ),
  ])
}

/** The root row: the main session, its model as far as a spawn reported it, and whether it has agents running. */
function orchestratorRow(ui: GraphicUi, agents: readonly Agent[]) {
  const { Box, Text } = ui
  const root = orchestratorOf(agents)
  const label = root.isActive ? 'running' : 'idle'
  const hue = root.isActive ? 'green' : 'gray'

  return (
    <Box flexDirection="row" alignItems="center" gap="sm">
      {art(ui, orchestratorSprite(root.isActive, ORCHESTRATOR_SIZE), `orchestrator ${label}`)}
      <Box flexGrow={1} flexShrink={1}>
        <Text size="footnote" wrap="truncate-end">
          <Text weight="semibold">orchestrator</Text>
          {root.model !== undefined && <Text color={MUTED}>{` ${modelName(root.model)}`}</Text>}
        </Text>
      </Box>
      <Box flexShrink={0}>{art(ui, pill(label, hue, { pulse: root.isActive }), label)}</Box>
    </Box>
  )
}

function agentRow(ui: GraphicUi, agent: Agent, depth: number, input: Input) {
  const { Box, Text } = ui
  const claim = modelClaim(agent, spawnOf(agent, input.snapshot))
  const look = lookOf(agent.status)
  const task = taskOf(agent)

  return (
    <Box flexDirection="column" marginLeft={Math.min(depth, 4) * 2}>
      <Box flexDirection="row" alignItems="center" gap="sm">
        {art(ui, agentSprite(agent.id, agent.status, 'orange', BOT_SIZE, agent.type), `${look.label} agent`)}
        <Box flexDirection="column" flexGrow={1} flexShrink={1}>
          {/* line 1: what it works on (its type when it has no label), one truncating line */}
          <Text size="footnote" weight="semibold" wrap="truncate-end">
            {task ?? agent.type}
          </Text>
          {/* line 2: the model claim and the type; the basis is a muted mark, not a word */}
          <Text size="caption" color={MUTED} wrap="truncate-end">
            <Text color={claim.basis === 'observed' || claim.basis === 'decided' ? ACCENT : undefined}>{modelName(claim.model)}</Text>
            {` ${claimMark(claim)} · ${agent.type}`}
          </Text>
        </Box>
        <Box flexDirection="row" alignItems="center" gap="sm" flexShrink={0}>
          {art(ui, pill(look.label, look.hue, { pulse: agent.status === 'running' }), look.label)}
          <Text size="footnote" tabular color={MUTED}>
            {elapsed((agent.endedAt ?? input.now) - agent.startedAt)}
          </Text>
        </Box>
      </Box>
      {input.isExpanded &&
        detailLines(agent, claim, input.snapshot).map(l => (
          <Text size="caption" mono color={MUTED} wrap="truncate-end">
            {l}
          </Text>
        ))}
    </Box>
  )
}
