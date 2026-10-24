// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Limits and input parsing for the flow tools. Pure: no engine calls, so every
// rule here is testable without a session. A bad input returns an error string.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { FlowPhase, FlowTask, FlowTier } from '../../../types'

export const MAX_TOTAL = 12
export const MAX_TITLE = 120
export const MAX_AFTER = 20
export const MAX_NOTE = 120
export const MAX_STEP_TOTAL = 99

export const PHASES: readonly FlowPhase[] = ['plan', 'design', 'delegate', 'review', 'close']
export const TIERS: readonly FlowTier[] = ['fable', 'heavy', 'careful', 'medium', 'light']

export type ProgressInput = {
  title?: string
  total?: number
  done?: number
  phase?: FlowPhase
  tasks?: FlowTask[]
  finished?: boolean
}

export type StepInput = { total?: number; done?: number; note?: string }

export type Parsed<T> = { ok: true; value: T } | { ok: false; error: string }

const fail = (error: string): { ok: false; error: string } => ({ ok: false, error })

const isRecord = (v: unknown): v is Record<string, unknown> =>
  typeof v === 'object' && v !== null && !Array.isArray(v)

const isCount = (v: unknown, max: number): v is number =>
  typeof v === 'number' && Number.isInteger(v) && v >= 0 && v <= max

function parseTitle(v: unknown, what: string): Parsed<string> {
  if (typeof v !== 'string') return fail(`${what} must be a string`)
  const title = v.trim()
  if (title === '' || title.length > MAX_TITLE) return fail(`${what} must be 1 to ${MAX_TITLE} characters`)
  return { ok: true, value: title }
}

function parseTask(v: unknown, i: number): Parsed<FlowTask> {
  if (!isRecord(v)) return fail(`tasks[${i}] must be an object`)
  const title = parseTitle(v.title, `tasks[${i}].title`)
  if (!title.ok) return title
  let tier: FlowTier | null = null
  if (v.tier !== undefined) {
    if (!TIERS.includes(v.tier as FlowTier)) return fail(`tasks[${i}].tier must be one of ${TIERS.join(', ')}`)
    tier = v.tier as FlowTier
  }
  let after: string[] = []
  if (v.after !== undefined) {
    if (!Array.isArray(v.after) || v.after.length > MAX_AFTER) return fail(`tasks[${i}].after must be a list of at most ${MAX_AFTER}`)
    for (const a of v.after) {
      const t = parseTitle(a, `tasks[${i}].after item`)
      if (!t.ok) return t
      after.push(t.value)
    }
  }
  return { ok: true, value: { title: title.value, tier, after } }
}

export function parseProgress(raw: unknown): Parsed<ProgressInput> {
  if (!isRecord(raw)) return fail('progress input must be an object')
  const out: ProgressInput = {}
  if (raw.title !== undefined) {
    const t = parseTitle(raw.title, 'title')
    if (!t.ok) return t
    out.title = t.value
  }
  if (raw.total !== undefined) {
    if (!isCount(raw.total, MAX_TOTAL)) return fail(`total must be an integer from 0 to ${MAX_TOTAL}`)
    out.total = raw.total
  }
  if (raw.done !== undefined) {
    if (!isCount(raw.done, MAX_TOTAL)) return fail(`done must be an integer from 0 to ${MAX_TOTAL}`)
    out.done = raw.done
  }
  if (raw.phase !== undefined) {
    if (!PHASES.includes(raw.phase as FlowPhase)) return fail(`phase must be one of ${PHASES.join(', ')}`)
    out.phase = raw.phase as FlowPhase
  }
  if (raw.finished !== undefined) {
    if (typeof raw.finished !== 'boolean') return fail('finished must be a boolean')
    out.finished = raw.finished
  }
  if (raw.tasks !== undefined) {
    if (!Array.isArray(raw.tasks) || raw.tasks.length > MAX_TOTAL) return fail(`tasks must be a list of at most ${MAX_TOTAL}`)
    const tasks: FlowTask[] = []
    for (const [i, t] of raw.tasks.entries()) {
      const p = parseTask(t, i)
      if (!p.ok) return p
      tasks.push(p.value)
    }
    out.tasks = tasks
  }
  return { ok: true, value: out }
}

export function parseStep(raw: unknown): Parsed<StepInput> {
  if (!isRecord(raw)) return fail('step input must be an object')
  const out: StepInput = {}
  for (const key of ['total', 'done'] as const) {
    const v = raw[key]
    if (v === undefined) continue
    if (!isCount(v, MAX_STEP_TOTAL)) return fail(`${key} must be an integer from 0 to ${MAX_STEP_TOTAL}`)
    out[key] = v
  }
  if (raw.note !== undefined) {
    if (typeof raw.note !== 'string' || raw.note.length > MAX_NOTE) return fail(`note must be a string of at most ${MAX_NOTE} characters`)
    out.note = raw.note
  }
  return { ok: true, value: out }
}
