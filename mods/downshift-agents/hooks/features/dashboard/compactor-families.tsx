// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Compactor card's "Top families" block on the terminal: one row per
// command family, its share of tokens cut as a meter, the percent, and the
// estimated tokens avoided. Rows are built to the columns they get, so none wraps.

import type { CompactorFamily } from '../../../types'
import { MUTED, WARN, fit, meter, tokens } from '../../shared/ui/format'
import { innerColumns } from './card'
import type { Ui } from './ui'

export const TOP_FAMILIES = 5

const NAME_MAX = 12
const BAR_MIN = 4

/** The share of a family's raw tokens the Compactor cut, in whole percent; 0 when nothing was measured. */
export function familyShare(f: CompactorFamily): number {
  return f.rawTokens > 0 ? Math.min(100, Math.round((f.avoidedTokens / f.rawTokens) * 100)) : 0
}

export const familyTokens = (f: CompactorFamily): string => `~${tokens(f.avoidedTokens)}*`

type Cells = { name: string; bar: number; pct: string; saved: string }

/** The cells of one row at `width` columns: name, meter (dropped when too narrow), percent, tokens. */
export function rowCells(f: CompactorFamily, nameWidth: number, savedWidth: number, width: number): Cells {
  const pct = `${familyShare(f)}%`.padStart(4)
  const saved = familyTokens(f).padStart(savedWidth)
  const room = width - nameWidth - pct.length - saved.length - 3
  return { name: fit(f.family, nameWidth).padEnd(nameWidth), bar: room >= BAR_MIN ? room : 0, pct, saved }
}

/** Nothing for no families, so an older binary leaves the card as it was. */
export function familiesBlock(ui: Ui, families: CompactorFamily[], columns: number) {
  const { Box, Text } = ui
  if (families.length === 0) {
    return []
  }
  const width = innerColumns(columns)
  const shown = families.slice(0, TOP_FAMILIES)
  const nameWidth = Math.min(NAME_MAX, Math.max(...shown.map(f => f.family.length)), Math.max(4, Math.floor(width / 3)))
  const savedWidth = Math.max(...shown.map(f => familyTokens(f).length))
  return [
    <Text color={MUTED} bold>
      Top families
    </Text>,
    ...shown.map(f => {
      const cells = rowCells(f, nameWidth, savedWidth, width)
      const m = meter(cells.bar, familyShare(f))
      return (
        <Box flexDirection="row" gap={1}>
          <Text wrap="truncate-end">{cells.name}</Text>
          {cells.bar > 0 && (
            <Text>
              <Text color="success">{m.fill}</Text>
              <Text color={MUTED}>{m.track}</Text>
            </Text>
          )}
          <Text color={MUTED}>{cells.pct}</Text>
          <Text color={WARN} bold>
            {cells.saved}
          </Text>
        </Box>
      )
    }),
  ]
}
