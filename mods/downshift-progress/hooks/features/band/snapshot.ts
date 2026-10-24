// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0
//
// What the band keeps of a panel answer, and when its report block is stale.
// Copies the binary's fields as given; no figure is computed here.

import type { BandPanel, BandReport, BandReportCache } from '../../../types'
import { REPORT_MAX_AGE_MS } from '../../shared/data'
import type { Panel, PanelReport } from '../../shared/data'

export const NO_BAND_REPORT: BandReportCache = { report: null, at: null }

export const snapshotPanel = (p: Panel): BandPanel => ({
  summary: p.summary,
  modelSwitched: p.modelSwitched,
  effortChanged: p.effortChanged,
  held: p.held,
  spawns: p.spawns.map(s => ({ state: s.state })),
})

const snapshotReport = (r: PanelReport): BandReport => ({
  tokensSaved: r.display.tokensSaved,
  tokensSavedShort: r.display.tokensSavedShort,
  scopeLabel: r.display.scopeLabel ?? r.scope.label,
})

/** Keeps the panel's report when it carries one; otherwise the cache stays. */
export function nextReportCache(cache: BandReportCache, p: Panel, now: number): BandReportCache {
  return p.report ? { report: snapshotReport(p.report), at: now } : cache
}

/** True when there is no report, or it is at least 30 s old (or from the future). */
export function reportIsStale(cache: BandReportCache, now: number): boolean {
  if (cache.report === null || cache.at === null) return true
  const age = now - cache.at
  return age < 0 || age >= REPORT_MAX_AGE_MS
}
