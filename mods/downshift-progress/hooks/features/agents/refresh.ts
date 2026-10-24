// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Figures cadence: one call to `downshift session --panel` at a time, on a spawn
// end and every REFRESH_EVERY_MS while any agent runs; the report block only when
// the cached one is older than 30 s. Fails open: no binary or no surface leaves the
// rows as they are.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { EngineInterface, Timer } from 'claude-code'
import type { AgentFigures } from '../../../types'
import { hostPort } from './host'
import { fetchPanel, rememberReport, reportOlderThan } from '../../shared/data'
import type { Panel } from '../../shared/data'
import { FINISHED_SHOW_MS, REFRESH_EVERY_MS } from '../../shared/roster/constants'
import { pruneFinished, readReportCache, readRunning, writeFigures, writeReportCache } from '../../shared/roster/state'

type Host = EngineInterface

let isInFlight = false
let ticker: Timer | null = null

/** Per-spawn figures by the panel's key; models are the binary's own words. */
export function figuresOf(panel: Panel): Record<string, AgentFigures> {
  const out: Record<string, AgentFigures> = {}
  for (const s of panel.spawns) {
    if (s.toolUse === null) continue
    out[s.toolUse] = { model: s.observedModel ?? s.toModel, state: s.state, savedShort: s.savedShort }
  }
  return out
}

async function sessionId($: Host): Promise<string | null> {
  try {
    return (await $.session.id()) || null
  } catch {
    return null
  }
}

/** Fetches once; a call while one is in flight is dropped (the next trigger covers it). */
export async function refreshFigures($: Host): Promise<void> {
  if (isInFlight) return
  isInFlight = true
  try {
    const now = await $.clock.now()
    const cache = await readReportCache($)
    const result = await fetchPanel(hostPort($), await sessionId($), { withReport: reportOlderThan(cache, now) })
    if (!result.ok) return
    await writeFigures($, figuresOf(result.panel))
    await writeReportCache($, rememberReport(cache, result.panel, now))
    redraw($)
  } catch {
    // Rows stay as they were.
  } finally {
    isInFlight = false
  }
}

function redraw($: Host): void {
  try {
    $.ui.invalidate('ui.render')
  } catch {
    // Nothing is drawing.
  }
}

/** Starts the 5 s refresh while an agent runs and stops it when none does. */
export async function syncTicker($: Host): Promise<void> {
  const count = (await readRunning($)).length
  if (count > 0 && ticker === null) {
    ticker = $.clock.every(REFRESH_EVERY_MS, () => void tick($))
  } else if (count === 0 && ticker !== null) {
    ticker.cancel()
    ticker = null
  }
}

async function tick($: Host): Promise<void> {
  try {
    await syncTicker($)
    if (ticker !== null) await refreshFigures($)
  } catch {
    // Next period tries again.
  }
}

/** Removes the finished rows once they expire, and redraws. One timer per ended run. */
export function scheduleExpiry($: Host): void {
  $.clock.after(FINISHED_SHOW_MS, () => {
    void (async () => {
      try {
        await pruneFinished($, await $.clock.now())
        redraw($)
      } catch {
        // The next read hides expired rows anyway.
      }
    })()
  })
}
