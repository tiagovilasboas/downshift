// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Flow state atoms and the read helpers the band and the agents pane use.
// Only this feature writes them (through tools.ts). The atoms and every update
// stay in this file: the engine's scan lists a module's writes from one file.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import { atom, read, update } from 'claude-code'
import type { StateDollar } from 'claude-code'
import type { Flow, FlowStep } from '../../../types'

const flowAtom = atom({ plugin: 'downshift-progress', key: 'flow' } as const, null)
const flowStepsAtom = atom({ plugin: 'downshift-progress', key: 'flowSteps' } as const, {})

type Reader = StateDollar

export function readFlow($: Reader): Promise<Flow | null> {
  return read($, flowAtom)
}

export function readFlowSteps($: Reader): Promise<Record<string, FlowStep>> {
  return read($, flowStepsAtom)
}

/** The step record of one worker, or undefined while it has reported none. */
export async function readStepOf($: Reader, agentId: string): Promise<FlowStep | undefined> {
  return (await readFlowSteps($))[agentId]
}

/** Done tasks over total as a whole percent, 0 when there is no total. */
export function flowPercent(flow: Flow | null): number {
  if (flow === null || flow.total === 0) return 0
  return Math.round((flow.done / flow.total) * 100)
}

export async function writeFlow($: StateDollar, flow: Flow, isNew: boolean): Promise<void> {
  await update($, flowAtom, () => flow)
  if (isNew) await update($, flowStepsAtom, () => ({}))
}

export async function writeStep($: StateDollar, agentId: string, step: FlowStep): Promise<void> {
  await update($, flowStepsAtom, all => ({ ...all, [agentId]: step }))
}
