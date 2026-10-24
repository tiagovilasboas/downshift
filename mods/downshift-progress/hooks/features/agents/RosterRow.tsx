// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// One agent row of the pane: glyph, title, then the facts the row has. The
// binary's figure (`savedShort`) is printed as it came.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { RowStatus } from '../../shared/format/glyph'
import { statusGlyph } from '../../shared/format/glyph'
import { elapsedSince, formatDuration } from '../../shared/format/duration'
import { truncateTitle } from '../../shared/format/title'
import type { Lang } from '../../shared/i18n/lang'
import { t } from '../../shared/i18n/t'
import type { RosterRow } from '../../shared/roster'
import { formatTokens } from '../../shared/roster'
import type { Palette } from '../../shared/theme/tokens'
import type { Ui } from './ui'

export type RowEnv = { ui: Ui; palette: Palette; lang: Lang; nowMs: number; columns: number; isAscii: boolean }

const colorOf = (status: RowStatus, p: Palette): string =>
  ({ running: p.accent, finished: p.success, failed: p.danger, planned: p.muted })[status]

/** The facts after the title, in a fixed order, only those the row has. */
export function detailsOf(row: RosterRow, env: Pick<RowEnv, 'lang' | 'nowMs'>): string {
  const parts: string[] = []
  if (row.tier) parts.push(row.tier)
  if (row.model) parts.push(row.model)
  parts.push(row.endedAt === null ? elapsedSince(row.startedAt, env.nowMs) : formatDuration(row.endedAt - row.startedAt))
  if (row.tokens > 0) parts.push(`${formatTokens(row.tokens)} ${t('tokens', env.lang).toLowerCase()}`)
  if (row.savedShort) parts.push(`↓ ${row.savedShort}`)
  return parts.join(' · ')
}

export function agentRow(row: RosterRow, env: RowEnv) {
  const { Box, Text } = env.ui
  const details = detailsOf(row, env)
  const room = Math.max(8, env.columns - details.length - 6)
  return (
    <Box key={row.id} flexDirection="row" gap={1}>
      <Text color={colorOf(row.status, env.palette)}>{statusGlyph(row.status, env.isAscii)}</Text>
      <Text color={env.palette.text}>{truncateTitle(row.title, room)}</Text>
      <Text color={env.palette.muted}>{details}</Text>
    </Box>
  )
}
