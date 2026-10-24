// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The header tile and the three sections of the pane. Pure drawing: every figure
// comes ready from the roster; nothing here adds or converts an accounting number.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { RenderChildren } from 'claude-code'
import type { FlowTask } from '../../../types'
import { t } from '../../shared/i18n/t'
import type { Roster, RosterRow } from '../../shared/roster'
import { truncateTitle } from '../../shared/format/title'
import { statusGlyph } from '../../shared/format/glyph'
import { agentRow } from './RosterRow'
import type { RowEnv } from './RosterRow'

/** "Tokens saved · <scope>" and the report's short figure, both as the binary wrote them. */
export function headerTile(report: Roster['report'], env: RowEnv) {
  const short = report?.display.tokensSavedShort
  if (!report || !short) return null
  const { Box, Text } = env.ui
  const scope = report.display.scopeLabel ?? report.scope.label
  const label = scope ? `${t('tokensSaved', env.lang)} · ${scope}` : t('tokensSaved', env.lang)
  return (
    <Box key="tile" flexDirection="column" borderStyle="round" borderColor={env.palette.muted} paddingX={1}>
      <Text color={env.palette.muted}>{label}</Text>
      <Text bold color={env.palette.accent}>{short}</Text>
    </Box>
  )
}

function plannedRow(task: FlowTask, env: RowEnv) {
  const { Box, Text } = env.ui
  const after = task.after.length > 0 ? ` ← ${task.after.join(', ')}` : ''
  const tail = [task.tier ?? '', after].join('').trim()
  return (
    <Box key={`planned:${task.title}`} flexDirection="row" gap={1}>
      <Text color={env.palette.muted}>{statusGlyph('planned', env.isAscii)}</Text>
      <Text color={env.palette.text}>{truncateTitle(task.title, Math.max(8, env.columns - tail.length - 6))}</Text>
      <Text color={env.palette.muted}>{tail}</Text>
    </Box>
  )
}

function section(env: RowEnv, key: 'running' | 'finished' | 'planned', rows: unknown[], draw: () => RenderChildren) {
  if (rows.length === 0) return null
  const { Box, Text } = env.ui
  return (
    <Box key={key} flexDirection="column">
      <Text bold color={env.palette.text}>{`${t(key, env.lang)} (${rows.length})`}</Text>
      {draw()}
    </Box>
  )
}

/** Running, Finished, Planned; an empty section is not drawn. */
export function sections(roster: Roster, env: RowEnv) {
  return [
    section(env, 'running', roster.running, () => roster.running.map((r: RosterRow) => agentRow(r, env))),
    section(env, 'finished', roster.finished, () => roster.finished.map((r: RosterRow) => agentRow(r, env))),
    section(env, 'planned', roster.planned, () => roster.planned.map(task => plannedRow(task, env))),
  ]
}

export function isEmpty(roster: Roster): boolean {
  return roster.running.length + roster.finished.length + roster.planned.length === 0
}
