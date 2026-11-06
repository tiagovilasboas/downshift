// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The building blocks of the dashboard where the surface draws Svg and type
// sizes (desktop, vscode, mobile): a titled panel, a label/value row and an
// Svg drawing. The terminal never reaches this file; it keeps card.tsx.

import type { Elements, RenderChildren } from 'claude-code'

import { MUTED } from '../../../shared/ui/format'
import { svgDoc } from '../../../shared/ui/svg/shapes'
import type { Drawing } from '../../../shared/ui/svg/shapes'

/** The elements every Svg-drawing surface carries. */
export type GraphicUi = Pick<Elements['desktop'], 'Box' | 'Text' | 'Button' | 'Svg'>

/**
 * The panel border: a neutral grey at 30% opacity, so it reads as a hairline
 * on the light and the dark theme alike. The engine has no border width, so
 * the lighter colour is what makes it finer.
 */
export const PANEL_BORDER = '#8080804D'

/**
 * The breathing room under a title: 6 px, between the xs and sm spacing tokens,
 * so it is an empty drawing of that height. More than the row spacing, so the
 * title reads as the head of its content.
 */
const TITLE_GAP = 6

function titleGap(ui: GraphicUi) {
  return spacer(ui, TITLE_GAP, 'title gap')
}

/** Vertical room of an exact height, finer than the spacing tokens: an empty drawing. */
export function spacer(ui: GraphicUi, height: number, alt: string) {
  return art(ui, { source: svgDoc(1, height, ''), width: 1, height }, alt)
}

/**
 * CSS px per cell of the surface's code font, rounded up. A bar is drawn at
 * most as wide as its slot, so a generous guess fills the slot exactly.
 */
const PX_PER_COLUMN = 8.5

export const pxOf = (columns: number): number => Math.round(Math.max(1, columns) * PX_PER_COLUMN)

/** An Svg at its own size, or with `fill`, as wide as its slot allows at its own height. */
export function art(ui: GraphicUi, d: Drawing, alt: string, fill = false) {
  const { Svg } = ui
  return fill ? <Svg source={d.source} alt={alt} height={d.height} /> : <Svg source={d.source} alt={alt} width={d.width} height={d.height} />
}

/**
 * A panel: an icon, the title in its section's color with its summary beside it
 * in one Text (so a search finds them together), an optional figure flush
 * right, then the content. Tight padding: the pane shares the window's height.
 */
export function panel(
  ui: GraphicUi,
  head: { title: string; color: string; summary?: string; icon: Drawing; aside?: RenderChildren },
  children: RenderChildren[],
) {
  const { Box, Text } = ui
  return (
    <Box flexDirection="column" borderStyle="round" borderColor={PANEL_BORDER} paddingX="sm" paddingY="sm">
      <Box flexDirection="row" justifyContent="space-between" alignItems="center" gap="sm">
        <Box flexDirection="row" alignItems="center" gap="sm" flexShrink={1}>
          {art(ui, head.icon, `${head.title.toLowerCase()} icon`)}
          <Text size="footnote" weight="semibold" color={head.color} wrap="truncate-end">
            {head.title}
            {head.summary !== undefined && <Text color={MUTED} weight="regular">{` ${head.summary}`}</Text>}
          </Text>
        </Box>
        {head.aside}
      </Box>
      {children.some(Boolean) && titleGap(ui)}
      {children}
    </Box>
  )
}

/** A label on the left, its value flush right in tabular digits. */
export function row(ui: GraphicUi, label: string, value: string, color?: string) {
  const { Box, Text } = ui
  return (
    <Box flexDirection="row" justifyContent="space-between" gap="sm">
      <Text size="footnote" color={MUTED} wrap="truncate-end">
        {label}
      </Text>
      <Text size="footnote" weight="semibold" tabular color={color}>
        {value}
      </Text>
    </Box>
  )
}

/** A small uppercase label over a figure. */
export function caption(ui: GraphicUi, text: string, wrap: 'wrap' | 'truncate-end' = 'truncate-end') {
  const { Text } = ui
  return (
    <Text size="caption" weight="semibold" color={MUTED} wrap={wrap}>
      {text}
    </Text>
  )
}
