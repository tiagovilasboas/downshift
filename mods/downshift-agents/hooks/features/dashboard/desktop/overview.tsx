// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The top of the dashboard where the surface draws Svg: the title with
// Downshift's connection pill, then three tiles of figures the pane already
// holds (agents running, the Compactor's estimate, the share of bytes it cut).
// A tile with no figure says so; none is filled in.

import type { Agent, DownshiftSnapshot } from '../../../../types'
import { agentCounts, cheaperShare } from '../../metrics/normalize'
import { ACCENT, FAIL, MUTED, WARN, tokens } from '../../../shared/ui/format'
import { bareMark } from '../../../shared/ui/svg/brand'
import type { Ground } from '../../../shared/ui/svg/brand'
import { icon } from '../../../shared/ui/svg/icons'
import type { Drawing } from '../../../shared/ui/svg/shapes'
import { pill } from '../../../shared/ui/svg/shapes'
import { PANEL_BORDER, art, caption } from './frame'
import type { GraphicUi } from './frame'

/** Below this many columns the tiles stack instead of sharing a row. */
const TILE_ROW_MIN = 54
/** The logo's height in px, as in the band. */
const LOGO_SIZE = 20
/** Empty px before the logo, inside its drawing, as in the band. */
const LOGO_INSET = 4

/** One row: the bare logo the band draws, the title, and the connection pill, which says live or not. */
export function header(ui: GraphicUi, snapshot: DownshiftSnapshot, ground: Ground) {
  const { Box, Text } = ui
  const isLive = snapshot.status === 'ok'
  const badge = isLive
    ? pill('LIVE', 'green', { pulse: true })
    : pill(snapshot.status === 'unknown' ? 'WAITING' : 'N/A', snapshot.status === 'unknown' ? 'gray' : 'orange')

  return (
    <Box flexDirection="row" justifyContent="space-between" alignItems="center" gap="sm">
      <Box flexDirection="row" alignItems="center" gap="xs" flexShrink={1}>
        {art(ui, bareMark(LOGO_SIZE, ground, LOGO_INSET), 'Downshift')}
        <Text size="heading" weight="semibold" wrap="truncate-end">
          DOWNSHIFT AGENTS
        </Text>
      </Box>
      {art(ui, badge, isLive ? 'live' : 'not live')}
    </Box>
  )
}

type Tile = { glyph: Drawing; label: string; value: string; color?: string; note: string; noteColor?: string }

export function tiles(ui: GraphicUi, agents: readonly Agent[], snapshot: DownshiftSnapshot, columns: number) {
  const { Box } = ui
  const counts = agentCounts(agents)
  const isOk = snapshot.status === 'ok'
  const c = isOk ? snapshot.compactor : undefined
  // the pane is Claude Code's: its own tokens, never summed with another harness
  const cheaper = isOk ? cheaperShare(snapshot.belowSession, 'claude-code') : undefined
  const missing = isOk ? 'no runs yet' : 'Downshift n/a'
  const isRow = columns >= TILE_ROW_MIN

  const all: Tile[] = [
    {
      glyph: icon('agents', 'green', 16),
      label: 'AGENTS',
      value: `${counts.active} / ${counts.total}`,
      color: counts.active > 0 ? ACCENT : undefined,
      note: counts.failed > 0 ? `${counts.failed} failed` : counts.stale > 0 ? `${counts.stale} stale` : 'running / this session',
      noteColor: counts.failed > 0 ? FAIL : undefined,
    },
    {
      glyph: icon('layers', 'orange', 16),
      label: 'TOKENS SAVED',
      value: c === undefined ? '—' : `~${tokens(c.estimatedTokens)}*`,
      color: c === undefined ? undefined : WARN,
      note: c === undefined ? missing : 'by the Compactor',
    },
    {
      glyph: icon('trend', 'green', 16),
      label: 'ON A CHEAPER MODEL',
      value: cheaper === undefined ? '—' : `${cheaper}%`,
      color: cheaper === undefined ? undefined : ACCENT,
      note: cheaper === undefined ? (isOk ? 'not measured yet' : missing) : 'of claude-code tokens',
    },
  ]

  return (
    <Box flexDirection={isRow ? 'row' : 'column'} gap="sm">
      {all.map(t => tile(ui, t, isRow))}
    </Box>
  )
}

function tile(ui: GraphicUi, t: Tile, isRow: boolean) {
  const { Box, Text } = ui
  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor={PANEL_BORDER}
      paddingX="sm"
      flexGrow={1}
      flexShrink={1}
      width={isRow ? '30%' : undefined}
    >
      <Box flexDirection="row" alignItems="center" gap="xs">
        {art(ui, t.glyph, `${t.label.toLowerCase()} icon`)}
        {caption(ui, t.label, 'wrap')}
      </Box>
      <Text size="heading" weight="semibold" tabular color={t.color} wrap="truncate-end">
        {t.value}
      </Text>
      <Text size="caption" color={t.noteColor ?? MUTED} wrap="truncate-end">
        {t.note}
      </Text>
    </Box>
  )
}
