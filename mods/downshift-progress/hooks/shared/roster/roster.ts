// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// What the band and the agents pane read: the roster as rows, joined with the
// per-spawn figures by the panel's tool_use key. Read-only; fails open to rows
// without figures.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { StateDollar } from 'claude-code'
import type { AgentFigures, AgentRun, FinishedRun, FlowTask } from '../../../types'
import type { PanelReport, ReportCache } from '../data'
import { shownRows } from './expiry'
import { readFigures, readFinished, readPlanned, readReportCache, readRunning } from './state'

export type RowStatus = 'running' | 'finished' | 'failed'

/** One agent row as drawn. `savedShort` and `state` are the binary's text, untouched. */
export type RosterRow = {
  id: string
  title: string
  type: string
  tier: AgentRun['tier']
  /** The binary's observed or target model when it has one, else the model resolved at spawn. */
  model: string | null
  startedAt: number
  /** Epoch ms the run ended; null while it runs. */
  endedAt: number | null
  status: RowStatus
  tokens: number
  state: string | null
  savedShort: string | null
}

export type Roster = {
  running: RosterRow[]
  finished: RosterRow[]
  planned: FlowTask[]
  /** The last report block, or null before any fetch / without the binary. */
  report: PanelReport | null
}

const NO_FIGURES: AgentFigures = { model: null, state: null, savedShort: null }

function rowOf(run: AgentRun, figures: Record<string, AgentFigures>, extra: { endedAt: number | null; status: RowStatus }): RosterRow {
  const f = (run.key !== '' && figures[run.key]) || NO_FIGURES
  return {
    id: run.toolUse,
    title: run.description,
    type: run.type,
    tier: run.tier,
    model: f.model ?? run.model,
    startedAt: run.startedAt,
    endedAt: extra.endedAt,
    status: extra.status,
    tokens: run.tokens,
    state: f.state,
    savedShort: f.savedShort,
  }
}

/** What the atoms hold; `buildRoster` turns it into rows. */
export type RosterSnapshot = {
  running: readonly AgentRun[]
  finished: readonly FinishedRun[]
  planned: readonly FlowTask[]
  figures: Record<string, AgentFigures>
  cache: ReportCache
}

/** The roster at `nowMs`, pure: finished rows past their expiry are not in it. */
export function buildRoster(snap: RosterSnapshot, nowMs: number): Roster {
  return {
    running: snap.running.map(r => rowOf(r, snap.figures, { endedAt: null, status: 'running' })),
    finished: shownRows(snap.finished, nowMs).map(r =>
      rowOf(r, snap.figures, { endedAt: r.finishedAt, status: r.isFailed ? 'failed' : 'finished' }),
    ),
    planned: [...snap.planned],
    report: snap.cache.report,
  }
}

export async function readRoster($: StateDollar, nowMs: number): Promise<Roster> {
  const [running, finished, planned, figures, cache] = await Promise.all([
    readRunning($),
    readFinished($),
    readPlanned($),
    readFigures($),
    readReportCache($),
  ])
  return buildRoster({ running, finished, planned, figures, cache }, nowMs)
}

/** How many agents run now; the band shows this without reading the whole roster. */
export async function runningCount($: StateDollar): Promise<number> {
  return (await readRunning($)).length
}
