// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// The last report block the mod has seen and when. Pure: callers hold the value
// and pass the clock in. The report is not per session (it is the accounting
// window), so the cache has no session key.

import type { Panel, PanelReport } from './panel'

/** The report is refetched once it is this old. */
export const REPORT_MAX_AGE_MS = 30_000

export type ReportCache = {
  readonly report: PanelReport | null
  /** When `report` was fetched, in epoch milliseconds. */
  readonly at: number | null
}

export const NO_REPORT: ReportCache = { report: null, at: null }

/** True when there is no report, or it is at least `maxAgeMs` old. */
export function reportOlderThan(
  cache: ReportCache,
  nowMs: number,
  maxAgeMs: number = REPORT_MAX_AGE_MS,
): boolean {
  if (cache.report === null || cache.at === null) return true
  const age = nowMs - cache.at
  return age < 0 || age >= maxAgeMs
}

/** Keeps the panel's report when it carries one; otherwise the cache is kept. */
export function rememberReport(cache: ReportCache, panel: Panel, nowMs: number): ReportCache {
  return panel.report ? { report: panel.report, at: nowMs } : cache
}
