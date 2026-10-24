// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Keeps the band's panel fresh: one `session --panel` call in flight at a time,
// `--no-report` unless the cached report is older than 30 s, and a 5 s tick
// while a flow or a subagent is active. Render never fetches (the engine denies a
// state write made from a render dispatch); it draws the cache.

import type { EngineInterface, Timer } from 'claude-code'

import type { BandPanel } from '../../../types'
import { readFlow } from '../flow'
import { readSessionId } from './env'
import { callPanel } from './panel-call'
import { nextReportCache, reportIsStale, snapshotPanel } from './snapshot'
import { readBandPanel, readBandReport, writeBand } from './state'

export const TICK_MS = 5_000
/** The binary writes a subagent's tokens at its stop hook, which may run just after `turn.complete`. */
export const RETRY_MS = 1_500

/** How many subagents are running now. The roster replaces the default through the band's parameter. */
export type LiveCount = (panel: BandPanel | null) => number

export const runningSpawns: LiveCount = panel => panel?.spawns.filter(s => s.state === 'running').length ?? 0

let inFlight: Promise<void> | null = null
let isPending = false
let tick: Timer | null = null

async function fetchOnce($: EngineInterface, live: LiveCount): Promise<void> {
  const now = await $.clock.now()
  const cache = await readBandReport($)
  const result = await callPanel($, await readSessionId($), reportIsStale(cache, now))
  if (result.ok) await writeBand($, snapshotPanel(result.panel), nextReportCache(cache, result.panel, now))
  else await writeBand($, null, cache)
  $.ui.invalidate('ui.render')
  await keepTicking($, live)
}

async function keepTicking($: EngineInterface, live: LiveCount): Promise<void> {
  const flow = await readFlow($)
  const isActive = (flow !== null && !flow.finished) || live(await readBandPanel($)) > 0
  if (!isActive || tick !== null) return
  tick = $.clock.after(TICK_MS, () => {
    tick = null
    void refresh($, live)
  })
}

/** Fetches once; a trigger that arrives while a call is in flight coalesces into one more call. */
export function refresh($: EngineInterface, live: LiveCount): Promise<void> {
  if (inFlight !== null) {
    isPending = true
    return inFlight
  }
  inFlight = fetchOnce($, live)
    .catch(() => undefined)
    .then(() => {
      inFlight = null
      if (!isPending) return
      isPending = false
      return refresh($, live)
    })
  return inFlight
}

/** A subagent ended: refresh now and once more shortly after. */
export function afterSubagent($: EngineInterface, live: LiveCount): void {
  void refresh($, live)
  $.clock.after(RETRY_MS, () => void refresh($, live))
}
