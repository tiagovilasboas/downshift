// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Line 1 of the band: the flow the main session registered through `progress`.
// title · phase · done/total + bar · one glyph per planned task.

import type { Flow, FlowStep } from '../../../types'
import { statusGlyph } from '../../shared/format/glyph'
import type { RowStatus } from '../../shared/format/glyph'
import type { Lang } from '../../shared/i18n/lang'
import { flowPercent } from '../flow'
import { phaseLabel } from './labels'
import { joinGroups } from './line'
import type { Line, Segment } from './line'

export const BAR_WIDTH = 8

/** The finished flow stays this long before the band drops it. */
export const FINISHED_LINGER_MS = 10_000

/** A compact bar of `BAR_WIDTH` cells; the flow's percent decides the filled ones. */
export function flowBar(flow: Flow, ascii: boolean): string {
  const filled = Math.floor((flowPercent(flow) * BAR_WIDTH) / 100)
  return (ascii ? '#' : '█').repeat(filled) + (ascii ? '-' : '░').repeat(BAR_WIDTH - filled)
}

/** Status of each planned task: a worker that reported a finished step, one still stepping, the rest planned. */
export function taskStatuses(flow: Flow, steps: Readonly<Record<string, FlowStep>>): RowStatus[] {
  const reported = Object.values(steps)
  const statuses = flow.tasks.map((task): RowStatus => {
    if (flow.finished) return 'finished'
    const step = reported.find(s => s.task === task.title)
    if (step === undefined) return 'planned'
    return step.total > 0 && step.done >= step.total ? 'finished' : 'running'
  })
  // The main session may count a task done without any worker step: fill the earliest planned ones.
  let missing = flow.done - statuses.filter(s => s === 'finished').length
  return statuses.map(s => (s === 'planned' && missing-- > 0 ? 'finished' : s))
}

const TOKEN_OF = { finished: 'success', running: 'text', failed: 'danger', planned: 'muted' } as const

export function flowLine(flow: Flow, steps: Readonly<Record<string, FlowStep>>, lang: Lang, ascii: boolean): Line {
  const title: Segment = { text: flow.title, token: 'text', bold: true }
  const phase: Line = flow.phase === null ? [] : [{ text: phaseLabel(flow.phase, lang), token: 'muted' }]
  const progress: Line = [
    { text: `${flow.done}/${flow.total} `, token: 'text' },
    { text: flowBar(flow, ascii), token: 'accent' },
  ]
  const glyphs: Line = taskStatuses(flow, steps).map(status => ({
    text: statusGlyph(status, ascii),
    token: TOKEN_OF[status],
  }))
  return joinGroups([[title], phase, progress, glyphs])
}
