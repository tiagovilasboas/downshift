// Copyright (c) 2026 Tiago Vilas Boas.
// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'claude-code/testing'

import { NO_REPORT, REPORT_MAX_AGE_MS, rememberReport, reportOlderThan } from '.'
import type { Panel, PanelReport } from './panel'
import { PANEL_NO_REPORT_JSON, TODAY_PANEL_JSON } from './panel-fixtures'
import { parsePanel } from './panel-parse'

const withReport = parsePanel(TODAY_PANEL_JSON) as Panel
const noReport = parsePanel(PANEL_NO_REPORT_JSON) as Panel

test('no report yet means older than any age', () => {
  expect(REPORT_MAX_AGE_MS).toBe(30_000)
  expect(reportOlderThan(NO_REPORT, 1_000)).toBe(true)
})

test('a report is fresh for under 30 s and older from exactly 30 s', () => {
  const cache = rememberReport(NO_REPORT, withReport, 100_000)
  expect(reportOlderThan(cache, 100_000)).toBe(false)
  expect(reportOlderThan(cache, 129_999)).toBe(false)
  expect(reportOlderThan(cache, 130_000)).toBe(true)
})

test('a clock that went backwards counts as older', () => {
  const cache = rememberReport(NO_REPORT, withReport, 100_000)
  expect(reportOlderThan(cache, 99_000)).toBe(true)
})

test('a panel without a report keeps the cache as it was', () => {
  const cache = rememberReport(NO_REPORT, withReport, 100_000)
  expect(rememberReport(cache, noReport, 200_000)).toBe(cache)
  expect(rememberReport(NO_REPORT, noReport, 200_000)).toBe(NO_REPORT)
})

test('the cached report is the parsed one and the max age can be given', () => {
  const cache = rememberReport(NO_REPORT, withReport, 5_000)
  expect(cache.report as PanelReport).toEqual(withReport.report as PanelReport)
  expect(reportOlderThan(cache, 6_000, 2_000)).toBe(false)
  expect(reportOlderThan(cache, 5_500, 500)).toBe(true)
})
