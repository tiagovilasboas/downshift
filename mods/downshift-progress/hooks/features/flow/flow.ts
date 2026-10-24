// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Pure flow transitions: merge a progress call into the flow, merge a step into
// a worker's record, match a worker to a planned task.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { Flow, FlowStep } from '../../../types'
import type { ProgressInput, StepInput } from './bounds'

export type Merged = { flow: Flow; isNew: boolean } | { error: string }

const EMPTY = (title: string): Flow => ({ title, phase: null, total: 0, done: 0, finished: false, tasks: [] })

/** A titled call that differs from the current flow, or follows a finished one, starts a new flow. */
export function mergeProgress(current: Flow | null, input: ProgressInput): Merged {
  const startsNew = input.title !== undefined && (current === null || current.finished || current.title !== input.title)
  const base = startsNew || current === null ? EMPTY(input.title ?? 'Flow') : current
  const next: Flow = { ...base }
  if (input.phase !== undefined) next.phase = input.phase
  if (input.tasks !== undefined) next.tasks = input.tasks
  if (input.total !== undefined) next.total = input.total
  else if (input.tasks !== undefined) next.total = Math.max(next.total, input.tasks.length)
  if (input.done !== undefined) next.done = input.done
  if (next.done > next.total) return { error: `done (${next.done}) cannot exceed total (${next.total})` }
  if (input.finished === true) {
    next.finished = true
    next.done = next.total
  }
  return { flow: next, isNew: startsNew }
}

export function mergeStep(previous: FlowStep | undefined, input: StepInput, task: string | null): FlowStep | { error: string } {
  const step: FlowStep = { done: 0, total: 0, note: '', task, ...previous }
  if (input.total !== undefined) step.total = input.total
  if (input.done !== undefined) step.done = input.done
  if (input.note !== undefined) step.note = input.note
  if (step.done > step.total) return { error: `done (${step.done}) cannot exceed total (${step.total})` }
  step.task = task ?? step.task
  return step
}

/** The planned task whose title equals the agent's Agent-call description. */
export function matchTask(flow: Flow | null, description: string | undefined): string | null {
  if (flow === null || description === undefined) return null
  const wanted = description.trim()
  return flow.tasks.find(t => t.title === wanted)?.title ?? null
}
