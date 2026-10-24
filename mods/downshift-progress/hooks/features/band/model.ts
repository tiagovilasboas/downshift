// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The band as data: which lines show, from the flow and the cached panel.
// Pure; the render hook reads the inputs and the view draws the lines.

import type { BandPanel, BandReport, Flow, FlowStep } from '../../../types'
import { flowLine } from './flow-line'
import type { Line } from './line'
import { summaryLine } from './summary-line'
import type { Look } from './env'

export type BandInputs = {
  /** Null once the finished flow has lingered its time. */
  flow: Flow | null
  steps: Readonly<Record<string, FlowStep>>
  /** Null when the binary is missing or the surface has no host commands: the flow line stands alone. */
  panel: BandPanel | null
  report: BandReport | null
  running: number
  look: Look
}

export function buildBand(i: BandInputs): Line[] {
  const lines: Line[] = []
  if (i.flow !== null) lines.push(flowLine(i.flow, i.steps, i.look.lang, i.look.ascii))
  if (i.panel !== null) lines.push(summaryLine(i.panel, i.report, i.running, i.look.lang))
  return lines.filter(line => line.length > 0)
}
