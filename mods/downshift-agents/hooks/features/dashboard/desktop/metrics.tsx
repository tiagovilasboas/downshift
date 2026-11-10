// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Compactor and Routing panels where the surface draws Svg. Kept apart and
// never summed, as on the terminal: the bytes the Compactor cut are measured
// and drawn as a bar of the raw bytes; tokens from them are an estimate; tokens
// below the session model are workload, not tokens saved.

import type { CompactorFigures, DownshiftSnapshot } from '../../../../types'
import { uncutNote } from '../../metrics/cut'
import { directLine, routingLines, routingSummary } from '../../metrics/normalize'
import { ACCENT, MUTED, WARN, bytes, tokens } from '../../../shared/ui/format'
import { icon } from '../../../shared/ui/svg/icons'
import { BAR_HEIGHT } from '../../../shared/ui/svg/progress'
import { bar } from '../../../shared/ui/svg/shapes'
import { ROUTING_NOTE, unavailableText } from '../metrics-section'
import { familyRows } from './compactor-families'
import { art, panel, pxOf, row } from './frame'
import type { GraphicUi } from './frame'

/** Collapsed, each panel keeps its headline figures; expanded adds the counts behind them. */
/** `pct` is the percent cut as shown: the bar and the number glide together. */
export function compactorPanel(ui: GraphicUi, snapshot: DownshiftSnapshot, pct: number | undefined, columns: number, isExpanded: boolean) {
  const { Text } = ui
  const glyph = icon('layers', 'orange')
  if (snapshot.status !== 'ok') {
    return panel(ui, { title: 'COMPACTOR', color: WARN, icon: glyph }, [
      <Text size="footnote" color={MUTED}>
        {unavailableText(snapshot)}
      </Text>,
    ])
  }
  const c = snapshot.compactor
  if (c === undefined) {
    return panel(ui, { title: 'COMPACTOR', color: WARN, icon: glyph }, [
      <Text size="footnote" color={MUTED}>
        No compacted command in this session yet.
      </Text>,
    ])
  }
  return panel(ui, { title: 'COMPACTOR', color: WARN, icon: glyph }, [
    art(ui, bar(pxOf(columns - 4), BAR_HEIGHT, pct, 'green'), pct === undefined ? 'no raw bytes measured' : `${pct}% of the raw bytes cut`, true),
    uncutNote(c) === undefined ? (
      row(ui, 'Output cut', pct === undefined ? bytes(c.savedBytes) : `${bytes(c.savedBytes)} (${pct}%)`, ACCENT)
    ) : (
      <Text size="footnote" color={MUTED} wrap="wrap">
        {uncutNote(c)}
      </Text>
    ),
    row(ui, 'Tokens saved', `~${tokens(c.estimatedTokens)}*`, WARN),
    ...familyRows(ui, c.families, columns, isExpanded),
    ...directRows(ui, c),
    isExpanded && row(ui, 'Runs', `${c.commands} · ${c.compacted} compacted`),
    isExpanded && row(ui, 'Raw (measured)', bytes(c.rawBytes)),
    isExpanded && (
      <Text size="caption" color={MUTED} wrap="truncate-end">
        {`${c.scope || 'this session'}`}
      </Text>
    ),
  ])
}

function directRows(ui: GraphicUi, c: CompactorFigures) {
  const { Text } = ui
  const d = directLine(c)
  return d === undefined
    ? []
    : [
        row(ui, d.label, d.value, WARN),
        <Text size="caption" color={MUTED} wrap="truncate-end">
          {c.direct?.line ?? ''}
        </Text>,
      ]
}

export function routingPanel(ui: GraphicUi, snapshot: DownshiftSnapshot, delegated: number, isExpanded: boolean) {
  const { Box, Text } = ui
  if (snapshot.status !== 'ok') {
    return panel(ui, { title: 'ROUTING', color: ACCENT, summary: `${delegated} delegated`, icon: icon('routing', 'green') }, [
      <Text size="footnote" color={MUTED}>
        {unavailableText(snapshot)}
      </Text>,
    ])
  }
  const r = snapshot.routing

  return panel(ui, { title: 'ROUTING', color: ACCENT, summary: routingSummary(snapshot.belowSession, r.belowChildren, r.children), icon: icon('routing', 'green') }, [
    // lighter-model tokens first: the headline, per harness, never summed
    ...routingLines(snapshot.belowSession, snapshot.efficiency).map(l => row(ui, l.label, l.value, l.isObserved ? ACCENT : MUTED)),
    isExpanded && row(ui, 'Delegated', String(delegated)),
    isExpanded && row(ui, 'Downshift decided', String(r.spawns.length)),
    isExpanded && row(ui, 'Model changed', String(r.modelSwitched)),
    isExpanded && row(ui, 'Effort changed · held', `${r.effortChanged} · ${r.held}`),
    isExpanded && r.line !== '' && (
      <Text size="caption" color={MUTED} wrap="truncate-end">
        {r.line}
      </Text>
    ),
    <Box flexDirection="row" alignItems="center" gap="sm">
      {art(ui, icon('info', 'gray', 14), 'note')}
      <Text size="caption" color={MUTED} italic wrap="wrap">
        {ROUTING_NOTE}
      </Text>
    </Box>,
  ])
}
