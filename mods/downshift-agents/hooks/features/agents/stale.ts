// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The sweep for agents that never reported an end: an interrupted session, a
// spawn the engine never named. A running agent silent past the threshold
// becomes `stale`: counted apart from running, never deleted, and closed or
// revived by any later event for it.

import type { Agent } from '../../../types'

/** Agents silent this long are no longer counted as running. */
export const STALE_AFTER_MS = 120_000

/** The next list with silent running agents marked stale. */
export function sweepStale(list: readonly Agent[], now: number, staleAfterMs = STALE_AFTER_MS): Agent[] {
  const isSilent = (a: Agent) => a.status === 'running' && now - (a.touchedAt ?? a.startedAt) > staleAfterMs
  return list.map(a => (isSilent(a) ? { ...a, status: 'stale' as const } : a))
}

/** Whether any agent is still running, so the sweep has something to watch. */
export const hasRunning = (list: readonly Agent[]): boolean => list.some(a => a.status === 'running')
