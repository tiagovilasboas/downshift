// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The dashboard's building blocks: a titled card, a label/value row and a
// meter. Box, Text and Button only, so every surface draws them; a narrow pane
// drops the frame and keeps the content.

import type { RenderChildren } from 'claude-code'

import { ACCENT, MUTED, meter } from '../../shared/ui/format'
import type { Ui } from './ui'

/** Below this many columns a card loses its frame: two border cells and two of padding cost too much. */
const FRAME_MIN = 32

/** The columns a card's content gets inside a pane `columns` wide. */
export function innerColumns(columns: number): number {
  return columns >= FRAME_MIN ? columns - 4 : columns
}

/**
 * A card: the title in bold with its summary beside it in one line (so a
 * search finds them together), then the content. Framed in a rounded, dim
 * border when the pane is wide enough.
 */
export function card(
  ui: Ui,
  head: { title: string; summary?: string; summaryColor?: string; columns: number },
  children: RenderChildren[],
) {
  const { Box, Text } = ui
  const framed = head.columns >= FRAME_MIN
  return (
    <Box
      flexDirection="column"
      borderStyle={framed ? 'round' : undefined}
      borderColor={framed ? MUTED : undefined}
      borderDimColor={framed}
      paddingX={framed ? 1 : 0}
    >
      <Text wrap="truncate-end">
        <Text bold>{head.title}</Text>
        {head.summary !== undefined && <Text color={head.summaryColor ?? MUTED}>{` ${head.summary}`}</Text>}
      </Text>
      {children}
    </Box>
  )
}

/** A label on the left, its value flush right. */
export function kv(ui: Ui, label: string, value: string, color?: string) {
  const { Box, Text } = ui
  return (
    <Box flexDirection="row" justifyContent="space-between" gap={1}>
      <Text wrap="truncate-end">{label}</Text>
      <Text color={color} bold={color !== undefined}>
        {value}
      </Text>
    </Box>
  )
}

/** A meter `cells` wide in the accent color, its empty track muted. */
export function meterView(ui: Ui, cells: number, percent: number | undefined, color = ACCENT) {
  const { Text } = ui
  const m = meter(cells, percent)
  return (
    <Text>
      <Text color={color}>{m.fill}</Text>
      <Text color={MUTED}>{m.track}</Text>
    </Text>
  )
}
