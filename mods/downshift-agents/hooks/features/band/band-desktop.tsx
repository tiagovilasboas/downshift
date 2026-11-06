// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The band's row where the surface draws Svg and type sizes (desktop): the
// logo, then the same segments as text, with a drawn bar where the terminal
// has its glyph meter. The terminal never reaches this file.

import type { Elements } from 'claude-code'

import { MUTED } from '../../shared/ui/format'
import { bareMark } from '../../shared/ui/svg/brand'
import type { Ground } from '../../shared/ui/svg/brand'
import { BAR_HEIGHT, shineBar } from '../../shared/ui/svg/progress'
import type { Drawing } from '../../shared/ui/svg/shapes'
import type { ShownProgress } from '../tasks/shown'
import type { Part, Segment } from './band'

/** The elements the desktop row draws. */
export type RowUi = Pick<Elements['desktop'], 'Box' | 'Text' | 'Svg'>

/** The bar stands where the glyph meter does: a fixed slot 64 px wide, at the standard bar height. */
const BAR_WIDTH = 64
const LOGO_SIZE = 12
/** Empty px before the logo, inside its drawing. */
const LOGO_INSET = 6


/** A run of text parts, or the place of the meter. */
type Piece = { kind: 'meter' } | { kind: 'text'; parts: Part[] }

/** The parts of a segment as runs of text with the meter's glyph cells folded into one stop. */
function piecesOf(parts: Part[]): Piece[] {
  const pieces: Piece[] = []
  for (const p of parts) {
    const last = pieces.at(-1)
    if (p.isMeter === true) {
      if (last?.kind !== 'meter') {
        pieces.push({ kind: 'meter' })
      }
    } else if (last?.kind === 'text') {
      last.parts.push(p)
    } else {
      pieces.push({ kind: 'text', parts: [p] })
    }
  }
  return pieces
}

/** A run without the spaces at its edges: the row's gap spaces the pieces. */
function trimmed(parts: Part[]): Part[] {
  const last = parts.length - 1
  return parts
    .map((p, i) => ({ ...p, text: i === 0 ? p.text.trimStart() : p.text }))
    .map((p, i) => (i === last ? { ...p, text: p.text.trimEnd() } : p))
    .filter(p => p.text !== '')
}

/** The brand's leading diamond; the logo stands in for it. */
const withoutDiamond = (text: string): string => text.replace(/^◆\s*/u, '')

function drawing(ui: RowUi, d: Drawing, alt: string) {
  const { Svg } = ui
  return <Svg source={d.source} alt={alt} width={d.width} height={d.height} />
}

function textRun(ui: RowUi, parts: Part[], key: string) {
  const { Text } = ui
  const shown = key === 'brand' ? parts.map(p => ({ ...p, text: withoutDiamond(p.text) })) : parts
  return (
    <Text size="footnote" wrap="truncate-end">
      {trimmed(shown).map(p => (
        <Text color={p.color} weight={p.bold === true ? 'semibold' : undefined}>
          {p.text}
        </Text>
      ))}
    </Text>
  )
}

function segmentView(ui: RowUi, s: Segment, tasks: ShownProgress, ground: Ground) {
  const { Box } = ui
  return (
    <Box flexDirection="row" alignItems="center" gap={s.key === 'brand' ? 'xs' : 'sm'} flexShrink={1}>
      {s.key === 'brand' && drawing(ui, bareMark(LOGO_SIZE, ground, LOGO_INSET), 'downshift agents logo')}
      {piecesOf(s.parts).map(piece =>
        piece.kind === 'meter'
          ? drawing(ui, shineBar(BAR_WIDTH, BAR_HEIGHT, tasks.total > 0 ? tasks.share * 100 : undefined), `${tasks.completed} of ${tasks.total} tasks done`)
          : textRun(ui, piece.parts, s.key),
      )}
    </Box>
  )
}

/** The whole row, unframed so it keeps every column: the segments split by a muted dot. */
export function desktopRow(ui: RowUi, segments: Segment[], tasks: ShownProgress, ground: Ground) {
  const { Box, Text } = ui
  return (
    <Box flexDirection="row" alignItems="center" gap="sm">
      {segments.map((s, i) => (
        <Box flexDirection="row" alignItems="center" gap="sm" flexShrink={1}>
          {i > 0 && (
            <Text size="footnote" color={MUTED}>
              ·
            </Text>
          )}
          {segmentView(ui, s, tasks, ground)}
        </Box>
      ))}
    </Box>
  )
}
