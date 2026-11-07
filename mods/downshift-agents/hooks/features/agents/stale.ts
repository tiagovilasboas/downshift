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

/**
 * A stale agent silent this long is no longer shown as active: no end will
 * come for it (an interrupted session, a spawn never named). It stays in the
 * full list, so nothing is deleted, but the active view stops carrying it.
 */
export const GONE_AFTER_MS = 600_000

/** Running, or stale but heard from recently enough to still be waited on. */
export const isLive = (a: Agent, now: number, goneAfterMs = GONE_AFTER_MS): boolean =>
  a.status === 'running' || (a.status === 'stale' && now - (a.touchedAt ?? a.startedAt) <= goneAfterMs)
