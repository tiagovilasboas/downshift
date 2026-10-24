// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// Keeps the planned list: the flow's tasks that no agent has started. A task is
// started when an agent's description equals its title.
//
// Downshift by Tiago Vilas Boas · https://github.com/tiagovilasboas/downshift

import type { StateDollar } from 'claude-code'
import type { FlowTier } from '../../../types'
import { readFlow } from '../flow'
import { readFinished, readRunning, readStarted, writePlanned, writeStarted } from '../../shared/roster/state'

/** Tier of the planned task titled `description`, or null. */
export async function tierOfTask($: StateDollar, description: string): Promise<FlowTier | null> {
  const flow = await readFlow($)
  return flow?.tasks.find(t => t.title === description)?.tier ?? null
}

/**
 * Recomputes `planned` from the flow and the descriptions of every run so far.
 * A new flow title forgets the started set, then runs on show seed it again.
 */
export async function syncPlanned($: StateDollar): Promise<void> {
  const flow = await readFlow($)
  const flowTitle = flow?.title ?? null
  const before = await readStarted($)
  const kept = before.flow === flowTitle ? before.titles : []
  const runs = [...(await readRunning($)), ...(await readFinished($))]
  const titles = [...new Set([...kept, ...runs.map(r => r.description)])]
  await writeStarted($, { flow: flowTitle, titles })
  await writePlanned($, flow ? flow.tasks.filter(t => !titles.includes(t.title)) : [])
}
