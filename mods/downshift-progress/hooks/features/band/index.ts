// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Band feature: the progress band above the prompt. Line 1 is the active flow,
// line 2 the Downshift summary. Text only, no Button. Fail-open: any error in the
// drawing leaves the engine's own band, and no binary leaves the flow line alone.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { On } from 'claude-code'

import { readFlow, readFlowSteps } from '../flow'
import { readLook } from './env'
import { afterSubagent, refresh, runningSpawns } from './feed'
import type { LiveCount } from './feed'
import { FINISHED_LINGER_MS } from './flow-line'
import { buildBand } from './model'
import { readBandPanel, readBandReport } from './state'
import { bandTree } from './view'

export type { LiveCount } from './feed'

/** `live` counts running subagents from the panel; the roster passes its own. */
export function registerBand(on: On, live: LiveCount = runningSpawns): void {
  let finished: { title: string; at: number } | null = null

  on('prompt.submit', ($, e, next) => {
    void refresh($, live)
    return next(e)
  })

  on('ui.render', { component: 'AbovePrompt' }, async ($, e, next) => {
    try {
      if (e.props.hasSurvey) return next(e)
      let flow = await readFlow($)
      if (flow?.finished) {
        const now = await $.clock.now()
        if (finished?.title !== flow.title) {
          finished = { title: flow.title, at: now }
          $.clock.after(FINISHED_LINGER_MS, () => $.ui.invalidate('ui.render'))
        }
        if (now - finished.at >= FINISHED_LINGER_MS) flow = null
      } else {
        finished = null
      }
      const panel = await readBandPanel($)
      const lines = buildBand({
        flow,
        steps: await readFlowSteps($),
        panel,
        report: panel === null ? null : (await readBandReport($)).report,
        running: live(panel),
        look: await readLook($),
      })
      if (lines.length === 0) return next(e)
      return bandTree($.ui.resolve(e), lines)
    } catch {
      return next(e)
    }
  })
}
