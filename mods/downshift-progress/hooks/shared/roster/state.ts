// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Roster atoms and every write to them. The atoms and every update stay in this
// file (the engine's scan lists a module's writes from one file). Transitions are
// small and pure where they can be: the hooks in features/agents call them.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import { atom, read, update } from 'claude-code'
import type { StateDollar } from 'claude-code'
import type { AgentFigures, AgentRun, FinishedRun, FlowTask, StartedTasks } from '../../../types'
import { NO_REPORT } from '../data'
import type { ReportCache } from '../data'
import { MAX_ROWS } from './constants'
import { shownRows } from './expiry'

const runningAtom = atom({ plugin: 'downshift-progress', key: 'running' } as const, [])
const finishedAtom = atom({ plugin: 'downshift-progress', key: 'finished' } as const, [])
const plannedAtom = atom({ plugin: 'downshift-progress', key: 'planned' } as const, [])
const startedAtom = atom({ plugin: 'downshift-progress', key: 'started' } as const, { flow: null, titles: [] })
const figuresAtom = atom({ plugin: 'downshift-progress', key: 'figures' } as const, {})
const reportAtom = atom({ plugin: 'downshift-progress', key: 'reportCache' } as const, NO_REPORT)

type Reader = StateDollar

export function readRunning($: Reader): Promise<AgentRun[]> {
  return read($, runningAtom)
}
export function readFinished($: Reader): Promise<FinishedRun[]> {
  return read($, finishedAtom)
}
export function readPlanned($: Reader): Promise<FlowTask[]> {
  return read($, plannedAtom)
}
export function readStarted($: Reader): Promise<StartedTasks> {
  return read($, startedAtom)
}
export function readFigures($: Reader): Promise<Record<string, AgentFigures>> {
  return read($, figuresAtom)
}
export function readReportCache($: Reader): Promise<ReportCache> {
  return read($, reportAtom)
}

/** Adds a running row; a repeated tool call id replaces its row. */
export async function startRun($: Reader, run: AgentRun): Promise<void> {
  await update($, runningAtom, rows => [...rows.filter(r => r.toolUse !== run.toolUse), run].slice(-MAX_ROWS))
}

/** Adds `tokens` to the running row of an agent; unknown ids are ignored. */
export async function addTokens($: Reader, agentId: string, tokens: number): Promise<void> {
  if (tokens <= 0) return
  await update($, runningAtom, rows =>
    rows.map(r => (r.agentId === agentId ? { ...r, tokens: r.tokens + tokens } : r)),
  )
}

/**
 * Moves the running row of an agent to finished at `nowMs`. Returns the row, or
 * null when no running row has that agent (a second end, or an unknown agent).
 */
export async function endRun($: Reader, agentId: string, nowMs: number, isFailed: boolean): Promise<FinishedRun | null> {
  const row = (await read($, runningAtom)).find(r => r.agentId === agentId)
  if (!row) return null
  const done: FinishedRun = { ...row, finishedAt: nowMs, isFailed }
  await update($, runningAtom, rows => rows.filter(r => r.agentId !== agentId))
  await update($, finishedAtom, rows => [...rows, done].slice(-MAX_ROWS))
  return done
}

/** Drops the finished rows that expired at `nowMs`. */
export async function pruneFinished($: Reader, nowMs: number): Promise<void> {
  await update($, finishedAtom, rows => shownRows(rows, nowMs))
}

export async function writePlanned($: Reader, tasks: FlowTask[]): Promise<void> {
  await update($, plannedAtom, () => tasks)
}

export async function writeStarted($: Reader, started: StartedTasks): Promise<void> {
  await update($, startedAtom, () => started)
}

export async function writeFigures($: Reader, figures: Record<string, AgentFigures>): Promise<void> {
  await update($, figuresAtom, () => figures)
}

export async function writeReportCache($: Reader, cache: ReportCache): Promise<void> {
  await update($, reportAtom, () => cache)
}
