// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The task progress panel where the surface draws Svg: the share done as a
// rounded bar, the task in progress on its own row, then the task list
// with one status mark per task when expanded.

import type { TaskBoard } from '../../../../types'
import { sourceLabel } from '../../tasks/board'
import type { ShownProgress } from '../../tasks/shown'
import { ACCENT, MUTED } from '../../../shared/ui/format'
import { lookOf } from '../../../shared/ui/status'
import { icon, statusMark } from '../../../shared/ui/svg/icons'
import { BAR_HEIGHT, progressBar } from '../../../shared/ui/svg/progress'
import { art, caption, panel, pxOf, spacer } from './frame'

/** Room between the progress bar and the NOW row, so the bar does not touch it. */
const BAR_GAP = 2
import type { GraphicUi } from './frame'

/** `p` is the progress as shown: the bar and the numbers glide together. */
export function tasksPanel(ui: GraphicUi, board: TaskBoard, p: ShownProgress, columns: number, isExpanded: boolean) {
  const { Box, Text } = ui
  const width = pxOf(columns - 4)
  if (p.total === 0) {
    return panel(ui, { title: 'TASK PROGRESS', color: ACCENT, summary: 'no tasks reported yet', icon: icon('tasks', 'green') }, [])
  }
  const percent = p.percent ?? 0
  const flags = [
    p.running > 0 ? `${p.running} running` : '',
    p.stale > 0 ? `${p.stale} stale` : '',
    p.blocked > 0 ? `${p.blocked} blocked` : '',
    p.failed > 0 ? `${p.failed} failed` : '',
    p.cancelled > 0 ? `${p.cancelled} cancelled` : '',
  ].filter(Boolean)
  const source = sourceLabel(board.source)
  const aside = (
    <Text size="footnote" weight="semibold" tabular color={ACCENT}>
      {`${p.completed} / ${p.total} · ${percent}%`}
    </Text>
  )

  return panel(ui, { title: 'TASK PROGRESS', color: ACCENT, summary: `${p.completed} of ${p.total} done`, icon: icon('tasks', 'green'), aside }, [
    art(ui, progressBar(width, BAR_HEIGHT, { total: p.total, completed: p.share * p.total, running: p.running, finished: p.share >= 1 }), `${p.completed} of ${p.total} tasks done`, true),
    p.current !== undefined && spacer(ui, BAR_GAP, 'bar gap'),
    p.current !== undefined && (
      <Box flexDirection="row" alignItems="center" gap="sm">
        <Box flexDirection="row" alignItems="center" gap="sm" flexShrink={0}>
          {art(ui, icon('code', 'green', 14), 'current task')}
          {caption(ui, 'NOW', 'wrap')}
        </Box>
        <Box flexGrow={1} flexShrink={1}>
          <Text size="footnote" wrap="truncate-end">
            {p.current}
          </Text>
        </Box>
      </Box>
    ),
    <Text size="caption" color={MUTED} wrap="truncate-end">
      {[...flags, source].join(' · ')}
    </Text>,
    isExpanded && (
      <Box flexDirection="column">
        {board.tasks.map(t => {
          const look = lookOf(t.status)
          return (
            <Box flexDirection="row" alignItems="center" gap="sm">
              {art(ui, statusMark(t.status, look.hue), look.label)}
              <Text size="footnote" color={t.status === 'cancelled' ? MUTED : undefined} wrap="truncate-end">
                {t.subject}
              </Text>
            </Box>
          )
        })}
      </Box>
    ),
  ])
}
