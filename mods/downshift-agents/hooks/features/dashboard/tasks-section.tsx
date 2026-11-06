// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The task progress card of the dashboard.

import type { TaskBoard } from '../../../types'
import { sourceLabel } from '../tasks/board'
import type { ShownProgress } from '../tasks/shown'
import { ACCENT, MUTED, fit } from '../../shared/ui/format'
import { lookOf } from '../../shared/ui/status'
import { card, innerColumns, meterView } from './card'
import type { Ui } from './ui'

/** `p` is the progress as shown: its meter and numbers glide together. */
export function tasksSection(ui: Ui, board: TaskBoard, p: ShownProgress, columns: number, isExpanded: boolean) {
  const { Text } = ui
  const inner = innerColumns(columns)
  if (p.total === 0) {
    return card(ui, { title: 'TASK PROGRESS', summary: '—', columns }, [
      meterView(ui, inner, undefined),
      <Text color={MUTED}>No tasks reported yet.</Text>,
    ])
  }
  const flags = [
    p.running > 0 ? `${p.running} running` : '',
    p.stale > 0 ? `${p.stale} stale` : '',
    p.blocked > 0 ? `${p.blocked} blocked` : '',
    p.failed > 0 ? `${p.failed} failed` : '',
    p.cancelled > 0 ? `${p.cancelled} cancelled` : '',
  ].filter(Boolean)
  const source = sourceLabel(board.source)

  return card(ui, { title: 'TASK PROGRESS', summary: `${p.completed}/${p.total} · ${p.percent ?? 0}%`, summaryColor: ACCENT, columns }, [
    meterView(ui, inner, p.share * 100),
    p.current !== undefined && (
      <Text wrap="truncate-end">
        <Text color={ACCENT}>▸ </Text>
        {fit(p.current, inner - 2)}
      </Text>
    ),
    <Text color={MUTED} wrap="truncate-end">
      {[...flags, source].join(' · ')}
    </Text>,
    ...(isExpanded
      ? board.tasks.map(t => {
          const look = lookOf(t.status)
          return (
            <Text wrap="truncate-end" dimColor={t.status === 'cancelled'}>
              <Text color={look.color}>{look.mark} </Text>
              {fit(t.subject, inner - 2)}
            </Text>
          )
        })
      : []),
  ])
}
