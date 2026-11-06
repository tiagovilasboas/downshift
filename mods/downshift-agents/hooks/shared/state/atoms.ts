// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The session values the mod keeps, and the only reads and writes of them.
// The engine's scan needs each atom spelled in the file that reads or writes
// it, so the other files go through these functions. Change a shape tag when
// its value's fields change: a reload then declines what an earlier version
// stored.

import { atom, read, update } from 'claude-code'
import type { EngineInterface } from 'claude-code'

import type { Agent, DownshiftSnapshot, Glides, TaskBoard, View } from '../../../types'

const agentsAtom = atom({ plugin: 'downshift-agents', key: 'agents' } as const, [] as Agent[], {
  shape: 'agents-1',
})

const boardAtom = atom(
  { plugin: 'downshift-agents', key: 'board' } as const,
  { source: 'none', tasks: [] } as TaskBoard,
  { shape: 'board-1' },
)

const downshiftAtom = atom(
  { plugin: 'downshift-agents', key: 'downshift' } as const,
  { status: 'unknown' } as DownshiftSnapshot,
  { shape: 'downshift-1' },
)

const viewAtom = atom(
  { plugin: 'downshift-agents', key: 'view' } as const,
  { isExpanded: false, filter: 'active', isBandHidden: false } as View,
  { shape: 'view-1' },
)

const glidesAtom = atom({ plugin: 'downshift-agents', key: 'glides' } as const, {} as Glides, {
  shape: 'glides-1',
})

export type Snapshot = { agents: Agent[]; board: TaskBoard; downshift: DownshiftSnapshot; view: View; glides: Glides }

/** Everything a view draws, read together. */
export async function readAll($: EngineInterface): Promise<Snapshot> {
  const [agents, board, downshift, view, glides] = await Promise.all([
    read($, agentsAtom),
    read($, boardAtom),
    read($, downshiftAtom),
    read($, viewAtom),
    read($, glidesAtom),
  ])
  return { agents: [...agents], board, downshift, view, glides }
}

export async function updateAgents($: EngineInterface, fn: (list: readonly Agent[]) => Agent[]): Promise<void> {
  await update($, agentsAtom, list => fn(list))
}

export async function updateBoard($: EngineInterface, fn: (board: TaskBoard) => TaskBoard): Promise<void> {
  await update($, boardAtom, board => fn(board))
}

export async function setDownshift($: EngineInterface, snapshot: DownshiftSnapshot): Promise<void> {
  await update($, downshiftAtom, () => snapshot)
}

export async function updateView($: EngineInterface, fn: (view: View) => View): Promise<void> {
  await update($, viewAtom, view => fn(view))
}

export async function readGlides($: EngineInterface): Promise<Glides> {
  return read($, glidesAtom)
}

export async function updateGlides($: EngineInterface, fn: (glides: Glides) => Glides): Promise<void> {
  await update($, glidesAtom, glides => fn(glides))
}
