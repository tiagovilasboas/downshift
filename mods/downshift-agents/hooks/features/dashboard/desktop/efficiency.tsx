// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Efficiency panel where the surface draws Svg: per harness, never summed,
// the premium capacity preserved first, then spawns by tier, completed and
// rework. Normal colours; no tokens, no money.

import type { DownshiftSnapshot } from '../../../../types'
import { efficiencyLines } from '../../metrics/efficiency'
import { DONE, MUTED } from '../../../shared/ui/format'
import { icon } from '../../../shared/ui/svg/icons'
import { EFFICIENCY_NOTE, NO_EFFICIENCY } from '../efficiency-section'
import { unavailableText } from '../metrics-section'
import { panel, row } from './frame'
import type { GraphicUi } from './frame'

/** Collapsed, each harness keeps its premium line; expanded adds tiers, completed and rework. */
export function efficiencyPanel(ui: GraphicUi, snapshot: DownshiftSnapshot, isExpanded: boolean) {
  const { Text } = ui
  const head = { title: 'EFFICIENCY', color: DONE, icon: icon('trend', 'gray') }
  if (snapshot.status !== 'ok') {
    return panel(ui, head, [
      <Text size="footnote" color={MUTED}>
        {unavailableText(snapshot)}
      </Text>,
    ])
  }
  const lines = efficiencyLines(snapshot.efficiency)
  if (lines.length === 0) {
    return panel(ui, head, [
      <Text size="footnote" color={MUTED}>
        {NO_EFFICIENCY}
      </Text>,
    ])
  }
  return panel(ui, head, [
    ...lines.flatMap(l => [
      <Text size="footnote" weight="semibold" wrap="truncate-end">
        {l.harness}
      </Text>,
      <Text size="footnote" color={l.isPremiumObserved ? undefined : MUTED} wrap="wrap">
        {`Premium preserved: ${l.premium}`}
      </Text>,
      isExpanded && row(ui, 'Spawns by tier', l.tiers),
      isExpanded && row(ui, 'Completed', l.completed),
      isExpanded && row(ui, 'Rework', l.rework),
    ]),
    <Text size="caption" color={MUTED} italic wrap="wrap">
      {EFFICIENCY_NOTE}
    </Text>,
  ])
}
