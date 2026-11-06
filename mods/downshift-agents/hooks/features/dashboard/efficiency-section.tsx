// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Efficiency card on the terminal: per harness, never summed, the premium
// capacity preserved, spawns by tier, completed and rework. Counts and shares
// only, in normal colours: no tokens (the Compactor card has them), no money.

import type { DownshiftSnapshot } from '../../../types'
import { efficiencyLines } from '../metrics/efficiency'
import { MUTED } from '../../shared/ui/format'
import { card, kv } from './card'
import { unavailableText } from './metrics-section'
import type { Ui } from './ui'

export const EFFICIENCY_NOTE = 'More work done, less premium capacity used. Last 30 days.'
export const NO_EFFICIENCY = 'No routed spawns in this window.'

export function efficiencySection(ui: Ui, snapshot: DownshiftSnapshot, columns: number) {
  const { Text } = ui
  if (snapshot.status !== 'ok') {
    return card(ui, { title: 'EFFICIENCY', columns }, [<Text color={MUTED}>{unavailableText(snapshot)}</Text>])
  }
  const lines = efficiencyLines(snapshot.efficiency)
  if (lines.length === 0) {
    return card(ui, { title: 'EFFICIENCY', columns }, [<Text color={MUTED}>{NO_EFFICIENCY}</Text>])
  }
  return card(ui, { title: 'EFFICIENCY', columns }, [
    ...lines.flatMap(l => [
      <Text bold wrap="truncate-end">
        {l.harness}
      </Text>,
      <Text color={l.isPremiumObserved ? undefined : MUTED} wrap="wrap">
        {`Premium preserved: ${l.premium}`}
      </Text>,
      kv(ui, 'Spawns by tier', l.tiers),
      kv(ui, 'Completed', l.completed),
      kv(ui, 'Rework', l.rework),
    ]),
    <Text color={MUTED} italic wrap="wrap">
      {EFFICIENCY_NOTE}
    </Text>,
  ])
}
