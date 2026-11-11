// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The Compactor and Routing cards, drawn apart and never summed: bytes the
// Compactor cut are measured (tokens from them are an estimate), and tokens
// that ran below the session model are workload, not tokens saved.

import type { CompactorFigures, DownshiftSnapshot } from '../../../types'
import { uncutNote } from '../metrics/cut'
import { directLine, routingLines, routingSummary } from '../metrics/normalize'
import { ACCENT, MUTED, WARN, bytes, tokens } from '../../shared/ui/format'
import { card, kv } from './card'
import { familiesBlock } from './compactor-families'
import type { Ui } from './ui'

export const ESTIMATE_NOTE = '* estimate: bytes cut ÷ 4'
export const ROUTING_NOTE = "Same work on a lighter model: it spares the top model's quota."

/** `pct` is the percent cut as shown, gliding toward the live figure. */
export function compactorSection(ui: Ui, snapshot: DownshiftSnapshot, pct: number | undefined, columns: number) {
  const { Text } = ui
  if (snapshot.status !== 'ok') {
    return card(ui, { title: 'COMPACTOR', columns }, [<Text color={MUTED}>{unavailableText(snapshot)}</Text>])
  }
  const c = snapshot.compactor
  if (c === undefined) {
    return card(ui, { title: 'COMPACTOR', columns }, [
      <Text color={MUTED}>No compacted command in this session yet.</Text>,
    ])
  }
  return card(ui, { title: 'COMPACTOR', summary: `~${tokens(c.estimatedTokens)}* tokens saved`, summaryColor: WARN, columns }, [
    <Text color={MUTED} wrap="truncate-end">
      {c.scope || 'this session'}
    </Text>,
    kv(ui, 'Runs', `${c.commands} · ${c.compacted} compacted`),
    kv(ui, 'Raw (measured)', bytes(c.rawBytes)),
    uncutNote(c) === undefined ? (
      kv(ui, 'Output cut', pct === undefined ? bytes(c.savedBytes) : `${bytes(c.savedBytes)} (${pct}%)`, WARN)
    ) : (
      <Text color={MUTED} wrap="wrap">
        {uncutNote(c)}
      </Text>
    ),
    kv(ui, 'Tokens saved', `~${tokens(c.estimatedTokens)}*`, c.estimatedTokens > 0 ? WARN : MUTED),
    ...familiesBlock(ui, c.families, columns),
    ...directBlock(ui, c),
  ])
}

function directBlock(ui: Ui, c: CompactorFigures) {
  const { Text } = ui
  const d = directLine(c)
  return d === undefined
    ? []
    : [
        kv(ui, d.label, d.value, WARN),
        <Text color={MUTED} wrap="truncate-end">
          {c.direct?.line ?? ''}
        </Text>,
      ]
}

export function routingSection(ui: Ui, snapshot: DownshiftSnapshot, delegated: number, columns: number) {
  const { Text } = ui
  if (snapshot.status !== 'ok') {
    return card(ui, { title: 'ROUTING', summary: `${delegated} delegated`, columns }, [
      <Text color={MUTED}>{unavailableText(snapshot)}</Text>,
    ])
  }
  const r = snapshot.routing

  return card(ui, { title: 'ROUTING', summary: routingSummary(snapshot.belowSession, r.belowChildren, r.children), summaryColor: ACCENT, columns }, [
    // lighter-model tokens first, per harness, never summed
    ...routingLines(snapshot.belowSession, snapshot.efficiency).map(l => kv(ui, l.label, l.value, l.isObserved ? ACCENT : MUTED)),
    kv(ui, 'Delegated', String(delegated)),
    kv(ui, 'Downshift decided', String(r.spawns.length)),
    kv(ui, 'Model changed', String(r.modelSwitched)),
    kv(ui, 'Effort changed · held', `${r.effortChanged} · ${r.held}`),
    r.line !== '' && (
      <Text color={MUTED} wrap="truncate-end">
        {r.line}
      </Text>
    ),
    <Text color={MUTED} italic wrap="wrap">
      {ROUTING_NOTE}
    </Text>,
  ])
}

export function unavailableText(s: DownshiftSnapshot): string {
  if (s.status === 'unavailable') {
    return `Downshift metrics unavailable: ${s.reason}.`
  }
  return 'Waiting for the first Downshift read.'
}
