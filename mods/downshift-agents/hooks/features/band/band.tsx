// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The progress band above the prompt: one line, useful with the pane closed.
// It yields to a survey, draws nothing before the session has anything to
// show, and keeps whatever other mods draw in the band beneath its own row.

import type { Elements, On } from 'claude-code'

import { agentCounts, workingNow } from '../metrics/normalize'
import { progressOf } from '../tasks/board'
import { shownProgress } from '../tasks/shown'
import { readAll } from '../../shared/state/atoms'
import { ACCENT, FAIL, MUTED, WARN, fit, meter, tokens } from '../../shared/ui/format'
import { groundOfSettings } from '../../shared/ui/ground'
import { desktopRow } from './band-desktop'

/** A run of one style inside a segment; `isMeter` marks a cell of the glyph meter, which a surface that draws Svg replaces with a bar. */
export type Part = { text: string; color?: string; bold?: boolean; isMeter?: true }

/** One band segment; `text` is its parts joined, `min` the width below which it is dropped. */
export type Segment = { key: string; text: string; parts: Part[]; min: number }

/** The elements the terminal's glyph row draws. */
type GlyphUi = Pick<Elements['terminal'], 'Box' | 'Text'>

/** The working segment shows from this width, with its task cut to this many cells. */
const WORKING_MIN = 110
const WORKING_CELLS = 24

const segment = (key: string, min: number, parts: Part[]): Segment => ({
  key,
  min,
  parts,
  text: parts.map(p => p.text).join(''),
})

export function bandSegments(
  input: {
    tasks: ReturnType<typeof progressOf>
    agents: ReturnType<typeof agentCounts>
    compact?: number
    isDownshiftOk: boolean
    working?: ReturnType<typeof workingNow>
  },
  columns: number,
): Segment[] {
  const { tasks, agents } = input
  const segments: Segment[] = [
    segment('brand', 0, [{ text: '◆ ', color: ACCENT }, { text: 'Downshift Agents', bold: true }]),
    segment('tasks', 40, tasksParts(tasks, columns >= 90 ? 8 : 5)),
    segment('agents', 60, [
      { text: 'Agents ' },
      // running now only: the session total lives in the pane, where it is labelled
      { text: `${agents.active}`, color: agents.active > 0 ? ACCENT : undefined, bold: true },
      { text: ' active' },
      ...(agents.failed > 0 ? [{ text: ` · ${agents.failed} failed`, color: FAIL }] : []),
    ]),
  ]
  if (input.working !== undefined) {
    // short, and only where the row has room: the task is cut, never the figures
    const { task, others } = input.working
    segments.push(
      segment('working', WORKING_MIN, [
        { text: '↳ ', color: MUTED },
        { text: fit(task, WORKING_CELLS) },
        ...(others > 0 ? [{ text: ` +${others}`, color: MUTED }] : []),
      ]),
    )
  }
  if (tasks.blocked > 0) {
    segments.push(segment('blocked', 80, [{ text: `${tasks.blocked} blocked`, color: WARN }]))
  }
  segments.push(
    input.isDownshiftOk && input.compact !== undefined
      ? segment('compact', 75, [{ text: 'Compact ' }, { text: `~${tokens(input.compact)}*`, color: WARN, bold: true }])
      : segment('compact', 75, [{ text: 'Downshift n/a', color: MUTED }]),
  )
  return segments.filter(s => columns >= s.min)
}

function tasksParts(tasks: ReturnType<typeof progressOf>, cells: number): Part[] {
  if (tasks.total === 0) {
    return [{ text: 'Tasks —', color: MUTED }]
  }
  const m = meter(cells, tasks.percent)
  return [
    { text: 'Tasks ' },
    { text: `${tasks.completed}/${tasks.total}`, bold: true },
    { text: ' ' },
    { text: m.fill, color: ACCENT, isMeter: true },
    { text: m.track, color: MUTED, isMeter: true },
    { text: ` ${tasks.percent ?? 0}%`, color: ACCENT },
  ]
}

export function drawBand(on: On): void {
  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    const below = await next(e)
    const { agents, board, downshift: snapshot, view, glides } = await readAll($)
    const counts = agentCounts(agents)
    const hasNothing = counts.total === 0 && board.tasks.length === 0 && snapshot.status === 'unknown'
    if (e.props.hasSurvey || view.isBandHidden || hasNothing) {
      return below
    }
    const tasks = shownProgress(progressOf(board), glides.tasks, await $.clock.now())
    const segments = bandSegments(
      {
        tasks,
        agents: counts,
        compact: snapshot.status === 'ok' ? snapshot.compactor?.estimatedTokens : undefined,
        isDownshiftOk: snapshot.status === 'ok',
        working: workingNow(agents),
      },
      e.props.bodyColumns,
    )
    // The surface decides, not the table: the engine's table can carry Svg on
    // the terminal too, and the terminal keeps its glyph row.
    if (e.surface === 'desktop' || e.surface === 'vscode' || e.surface === 'mobile') {
      const ground = await groundOfSettings($)
      const ui = $.ui.resolve(e)
      const { Box } = ui
      return (
        <Box flexDirection="column">
          {desktopRow(ui, segments, tasks, ground)}
          {below}
        </Box>
      )
    }
    const ui = $.ui.resolve(e)
    const { Box } = ui
    return (
      <Box flexDirection="column">
        {glyphRow(ui, segments)}
        {below}
      </Box>
    )
  })
}

/** The terminal's row: one line of styled runs split by a muted rule. */
function glyphRow(ui: GlyphUi, segments: Segment[]) {
  const { Box, Text } = ui
  return (
    <Box>
      <Text wrap="truncate-end">
        {segments.map((s, i) => (
          <Text>
            {i > 0 && <Text color={MUTED}> │ </Text>}
            {s.parts.map(p => (
              <Text color={p.color} bold={p.bold}>
                {p.text}
              </Text>
            ))}
          </Text>
        ))}
      </Text>
    </Box>
  )
}
