// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Compactor panel's family rows where the surface draws Svg: the family
// and its tokens avoided over a green bar of the share cut. Top 3 collapsed,
// top 5 expanded.

import type { CompactorFamily } from '../../../../types'
import { WARN } from '../../../shared/ui/format'
import { BAR_HEIGHT } from '../../../shared/ui/svg/progress'
import { bar } from '../../../shared/ui/svg/shapes'
import { TOP_FAMILIES, familyShare, familyTokens } from '../compactor-families'
import { art, caption, pxOf, row } from './frame'
import type { GraphicUi } from './frame'

export const COLLAPSED_FAMILIES = 3

export function familyRows(ui: GraphicUi, families: CompactorFamily[], columns: number, isExpanded: boolean) {
  if (families.length === 0) {
    return []
  }
  const shown = families.slice(0, isExpanded ? TOP_FAMILIES : COLLAPSED_FAMILIES)
  return [
    caption(ui, 'TOP FAMILIES'),
    ...shown.flatMap(f => [
      row(ui, `${f.family} · ${familyShare(f)}%`, familyTokens(f), WARN),
      art(ui, bar(pxOf(columns - 4), BAR_HEIGHT, familyShare(f), 'green'), `${f.family}: ${familyShare(f)}% of its tokens cut`, true),
    ]),
  ]
}
